package session

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type ClaimPushBatchResult struct {
	LuaResult   ClaimPushBatchLuaResult
	AssignedUSN int64
}

type ClaimPushBatchLuaResult int64

const (
	ClaimPushBatchLuaErr ClaimPushBatchLuaResult = iota
	ClaimPushBatchLuaOK
	ClaimPushBatchLuaSessionNotFound
	ClaimPushBatchLuaSessionIDMismatch
	ClaimPushBatchLuaBatchSeqMismatch
	ClaimPushBatchLuaStateMismatch
)

//go:embed scripts/claim_push_batch.lua
var claimPushBatchLua string
var claimPushBatchScript = redis.NewScript(claimPushBatchLua)

func (s *SessionStore) ClaimPushBatch(ctx context.Context, userID, sessionID string, curBatchSeq int32) (ClaimPushBatchResult, error) {
	rawResult, err := claimPushBatchScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		curBatchSeq,
		syncSessionTTLSecs,
	).Int64Slice()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to claim push batch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq, "error", err)
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, err
	}

	if len(rawResult) != 2 {
		s.logger.ErrorCtx(ctx, "invalid claim push batch result", "result", rawResult)
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, fmt.Errorf("invalid claim push batch result: %v", rawResult)
	}

	result := ClaimPushBatchResult{
		LuaResult:   ClaimPushBatchLuaResult(rawResult[0]),
		AssignedUSN: rawResult[1],
	}

	switch result.LuaResult {
	case ClaimPushBatchLuaOK:
		return result, nil
	case ClaimPushBatchLuaSessionNotFound:
		s.logger.InfoCtx(ctx, "sync session not found", "userID", userID)
		return result, nil
	case ClaimPushBatchLuaSessionIDMismatch:
		s.logger.InfoCtx(ctx, "sync session id mismatch", "userID", userID, "sessionID", sessionID)
		return result, nil
	case ClaimPushBatchLuaBatchSeqMismatch:
		s.logger.InfoCtx(ctx, "sync batch seq mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	case ClaimPushBatchLuaStateMismatch:
		s.logger.InfoCtx(ctx, "sync session state mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	default:
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown claim push batch result", "result", result.LuaResult, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown claim push batch result and failed to get session for printing", "userID", userID, "error", err)
		}
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, fmt.Errorf("unknown claim push batch result: %d", result.LuaResult)
	}
}
