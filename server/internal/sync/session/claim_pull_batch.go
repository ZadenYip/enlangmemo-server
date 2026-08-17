package session

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type ClaimPullBatchResult struct {
	LuaResult                   ClaimPullBatchLuaResult
	SyncCursorUSN               int64
	SrvSyncCursorUSNAtHandshake int64

	// 还没拉取完的实体 type 队列，例如：1,2,3,4,5,6,7 如果为空则是 ""
	// 实体 type 范围具体见 sync_units.entity_type 里的 COMMENT
	PullEntityQueue string
}

type ClaimPullBatchLuaResult int64

const (
	ClaimPullBatchLuaErr ClaimPullBatchLuaResult = iota
	ClaimPullBatchLuaOK
	ClaimPullBatchLuaSessionNotFound
	ClaimPullBatchLuaSessionIDMismatch
	ClaimPullBatchLuaBatchSeqMismatch
	ClaimPullBatchLuaStateMismatch
)

//go:embed scripts/claim_pull_batch.lua
var claimPullBatchLua string
var claimPullBatchScript = redis.NewScript(claimPullBatchLua)

func (s *SessionStore) ClaimPullBatch(ctx context.Context, userID int64, sessionID string, curBatchSeq int32) (ClaimPullBatchResult, error) {
	rawResult, err := claimPullBatchScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		curBatchSeq,
		syncSessionTTLSecs,
	).Slice()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to claim pull batch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq, "error", err)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, err
	}

	if len(rawResult) != 4 {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch result", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch result: %v", rawResult)
	}
	luaResult, ok := rawResult[0].(int64)
	if !ok {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch lua result type", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch lua result type: %T", rawResult[0])
	}
	syncCursorUSN, ok := rawResult[1].(int64)
	if !ok {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch sync cursor usn type", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch sync cursor usn type: %T", rawResult[1])
	}
	srvSyncCursorUSNAtHandshake, ok := rawResult[2].(int64)
	if !ok {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch server cursor type", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch server cursor type: %T", rawResult[2])
	}
	pullEntityQueue, ok := rawResult[3].(string)
	if !ok {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch entity queue type", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch entity queue type: %T", rawResult[3])
	}

	result := ClaimPullBatchResult{
		LuaResult:                   ClaimPullBatchLuaResult(luaResult),
		SyncCursorUSN:               syncCursorUSN,
		SrvSyncCursorUSNAtHandshake: srvSyncCursorUSNAtHandshake,
		PullEntityQueue:             pullEntityQueue,
	}

	switch result.LuaResult {
	case ClaimPullBatchLuaOK:
		return result, nil
	case ClaimPullBatchLuaSessionNotFound:
		s.logger.InfoCtx(ctx, "sync session not found", "userID", userID)
		return result, nil
	case ClaimPullBatchLuaSessionIDMismatch:
		s.logger.InfoCtx(ctx, "sync session id mismatch", "userID", userID, "sessionID", sessionID)
		return result, nil
	case ClaimPullBatchLuaBatchSeqMismatch:
		s.logger.InfoCtx(ctx, "sync batch seq mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	case ClaimPullBatchLuaStateMismatch:
		s.logger.InfoCtx(ctx, "sync session state mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	default:
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown claim pull batch result", "result", result.LuaResult, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown claim pull batch result and failed to get session for printing", "userID", userID, "error", err)
		}
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("unknown claim pull batch result: %d", result.LuaResult)
	}
}
