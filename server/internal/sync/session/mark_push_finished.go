package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type MarkPushFinishedResult int64

const (
	MarkPushFinishedErr MarkPushFinishedResult = iota
	MarkPushFinishedOK
	MarkPushFinishedSessionNotFound
	MarkPushFinishedSessionIDMismatch
)

//go:embed scripts/mark_push_finished.lua
var markPushFinishedLua string
var markPushFinishedScript = redis.NewScript(markPushFinishedLua)

func (s *SessionStore) MarkPushFinished(ctx context.Context, userID, sessionID string) error {
	result, err := markPushFinishedScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		syncSessionTTLSecs,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run mark push finished script", "userID", userID, "sessionID", sessionID, "error", err)
		return err
	}

	switch MarkPushFinishedResult(result) {
	case MarkPushFinishedOK:
		return nil
	case MarkPushFinishedSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when marking push finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when marking push finished")
	case MarkPushFinishedSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when marking push finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when marking push finished")
	default:
		s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown mark push finished result and failed to get session for printing", "userID", userID, "error", err)
		}
		return fmt.Errorf("unknown mark push finished result: %d", result)
	}
}
