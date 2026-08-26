package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
)

type MarkPushFinishedResult int64

const (
	MarkPushFinishedErr MarkPushFinishedResult = iota
	MarkPushFinishedOK
	MarkPushFinishedSessionNotFound
	MarkPushFinishedSessionIDMismatch
	MarkPushFinishedBatchSeqMismatch
	MarkPushFinishedStateMismatch
)

//go:embed scripts/mark_push_finished.lua
var markPushFinishedLua string
var markPushFinishedScript = redis.NewScript(markPushFinishedLua)

func (s *SessionStore) MarkPushFinished(ctx context.Context, userID int64, sessionID string, curBatchSeq int32) error {
	result, err := markPushFinishedScript.Run(
		ctx,
		s.rdb,
		[]string{RdbSessionKey(userID)},
		sessionID,
		curBatchSeq,
		syncSessionTTLSecs,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run mark push finished script", "userID", userID, "sessionID", sessionID, "batchSeq", curBatchSeq, "error", err)
		return connect.NewError(connect.CodeInternal, nil)
	}

	switch MarkPushFinishedResult(result) {
	case MarkPushFinishedOK:
		s.logger.InfoCtx(ctx, "mark push finished success")
		return nil
	case MarkPushFinishedSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when marking push finished", "userID", userID, "sessionID", sessionID, "batchSeq", curBatchSeq)
		const msg = "sync session not found when marking push finished"
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	case MarkPushFinishedSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when marking push finished", "userID", userID, "sessionID", sessionID, "batchSeq", curBatchSeq)
		const msg = "sync session id mismatch when marking push finished"
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	case MarkPushFinishedBatchSeqMismatch:
		s.logger.ErrorCtx(ctx, "sync batch seq mismatch when marking push finished", "userID", userID, "sessionID", sessionID, "batchSeq", curBatchSeq)
		const msg = "sync batch seq mismatch when marking push finished"
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	case MarkPushFinishedStateMismatch:
		s.logger.ErrorCtx(ctx, "sync session state mismatch when marking push finished", "userID", userID, "sessionID", sessionID, "batchSeq", curBatchSeq)
		const msg = "sync session state mismatch when marking push finished"
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
	default:
		s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result, "session", session)
		} else {
			const msg = "unknown mark push finished result and failed to get session for printing"
			s.logger.ErrorCtx(ctx, msg, "userID", userID, "error", err)
		}
		return fmt.Errorf("unknown mark push finished result: %d", result)
	}
}
