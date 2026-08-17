package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type MarkPullEntityFinishedResult int64

const (
	MarkPullEntityFinishedErr MarkPullEntityFinishedResult = iota
	MarkPullEntityFinishedOK
	MarkPullEntityFinishedSessionNotFound
	MarkPullEntityFinishedSessionIDMismatch
)

//go:embed scripts/mark_pull_entity_finished.lua
var markPullEntityFinishedLua string
var markPullEntityFinishedScript = redis.NewScript(markPullEntityFinishedLua)

func (s *SessionStore) MarkPullEntityFinished(ctx context.Context, userID int64, sessionID, remainingEntities string) error {
	result, err := markPullEntityFinishedScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		remainingEntities,
		syncSessionTTLSecs,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run mark pull entity finished script", "userID", userID, "sessionID", sessionID, "remainingPullEntityQueue", remainingEntities, "error", err)
		return err
	}

	switch MarkPullEntityFinishedResult(result) {
	case MarkPullEntityFinishedOK:
		return nil
	case MarkPullEntityFinishedSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when marking pull entity finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when marking pull entity finished")
	case MarkPullEntityFinishedSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when marking pull entity finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when marking pull entity finished")
	default:
		s.logger.ErrorCtx(ctx, "unknown mark pull entity finished result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown mark pull entity finished result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown mark pull entity finished result and failed to get session for printing",
				"userID", userID, "error", err, "result", result,
			)
		}
		return fmt.Errorf("unknown mark pull entity finished result %d", result)
	}
}
