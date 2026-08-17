package session

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type ClaimPullBatchResult struct {
	LuaResult                      ClaimPullBatchLuaResult
	SyncCursorUSN                  int64
	ServerSyncCursorUSNAtHandshake int64
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

func (s *SessionStore) ClaimPullBatch(ctx context.Context, userID, sessionID string, curBatchSeq int32) (ClaimPullBatchResult, error) {
	rawResult, err := claimPullBatchScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		curBatchSeq,
		syncSessionTTLSecs,
	).Int64Slice()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to claim pull batch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq, "error", err)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, err
	}

	if len(rawResult) != 3 {
		s.logger.ErrorCtx(ctx, "invalid claim pull batch result", "result", rawResult)
		return ClaimPullBatchResult{LuaResult: ClaimPullBatchLuaErr}, fmt.Errorf("invalid claim pull batch result: %v", rawResult)
	}

	result := ClaimPullBatchResult{
		LuaResult:                      ClaimPullBatchLuaResult(rawResult[0]),
		SyncCursorUSN:                  rawResult[1],
		ServerSyncCursorUSNAtHandshake: rawResult[2],
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
