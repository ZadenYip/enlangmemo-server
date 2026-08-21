package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type UpdatePullProgressResult int64

const (
	UpdatePullProgressErr UpdatePullProgressResult = iota
	UpdatePullProgressOK
	UpdatePullProgressSessionNotFound
	UpdatePullProgressSessionIDMismatch
)

//go:embed scripts/update_pull_progress.lua
var updatePullProgressLua string
var updatePullProgressScript = redis.NewScript(updatePullProgressLua)

func (s *SessionStore) UpdatePullProgress(ctx context.Context, userID int64, sessionID string, remainingPullEntityQueue string, syncCursorUSN int64) error {
	result, err := updatePullProgressScript.Run(
		ctx,
		s.rdb,
		[]string{RdbSessionKey(userID)},
		sessionID,
		remainingPullEntityQueue,
		syncCursorUSN,
		syncSessionTTLSecs,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run update pull progress script", "userID", userID, "sessionID", sessionID, "remainingPullEntityQueue", remainingPullEntityQueue, "syncCursorUSN", syncCursorUSN, "error", err)
		return err
	}

	switch UpdatePullProgressResult(result) {
	case UpdatePullProgressOK:
		return nil
	case UpdatePullProgressSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when updating pull progress", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when updating pull progress")
	case UpdatePullProgressSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when updating pull progress", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when updating pull progress")
	default:
		s.logger.ErrorCtx(ctx, "unknown update pull progress result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown update pull progress result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown update pull progress result and failed to get session for printing",
				"userID", userID, "error", err, "result", result,
			)
		}
		return fmt.Errorf("unknown update pull progress result %d", result)
	}
}
