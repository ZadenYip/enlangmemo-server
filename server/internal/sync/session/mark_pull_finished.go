package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type MarkPullFinishedResult int64

const (
	MarkPullFinishedErr MarkPullFinishedResult = iota
	MarkPullFinishedOK
	MarkPullFinishedSessionNotFound
	MarkPullFinishedSessionIDMismatch
)

//go:embed scripts/mark_pull_finished.lua
var markPullFinishedLua string
var markPullFinishedScript = redis.NewScript(markPullFinishedLua)

func (s *SessionStore) MarkPullFinished(ctx context.Context, userID, sessionID string, syncCursor int64) error {
	result, err := markPullFinishedScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		syncCursor,
		int64(syncSessionTTLSecs),
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run mark pull finished script", "userID", userID, "sessionID", sessionID, "syncCursor", syncCursor, "error", err)
		return err
	}

	switch MarkPullFinishedResult(result) {
	case MarkPullFinishedOK:
		return nil
	case MarkPullFinishedSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when marking pull finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when marking pull finished")
	case MarkPullFinishedSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when marking pull finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when marking pull finished")
	default:
		s.logger.ErrorCtx(ctx, "unknown mark pull finished result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown mark pull finished result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown mark pull finished result and failed to get session for printing",
				"userID", userID, "error", err, "result", result,
			)
		}
		return fmt.Errorf("unknown mark pull finished result %d", result)
	}
}
