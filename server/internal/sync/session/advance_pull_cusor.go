package session

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type AdvPullCursorResult int64

const (
	AdvPullCursorErr AdvPullCursorResult = iota
	AdvPullCursorOK
	AdvPullCursorSessionNotFound
	AdvPullCursorSessionIDMismatch
)

//go:embed scripts/advance_pull_cursor.lua
var advPullCursorLua string
var advPullCursorScript = redis.NewScript(advPullCursorLua)

func (s *SessionStore) AdvancePullCursor(ctx context.Context, userID, sessionID string, newSyncCursorUSN int64) error {
	result, err := advPullCursorScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		newSyncCursorUSN,
		syncSessionTTLSecs,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to run advance pull cursor script", "userID", userID, "sessionID", sessionID, "newSyncCursorUSN", newSyncCursorUSN, "error", err)
		return err
	}

	switch AdvPullCursorResult(result) {
	case AdvPullCursorOK:
		return nil
	case AdvPullCursorSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when advancing pull cursor", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when advancing pull cursor")
	case AdvPullCursorSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when advancing pull cursor", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when advancing pull cursor")
	default:
		s.logger.ErrorCtx(ctx, "unknown advance pull cursor result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown advance pull cursor result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown advance pull cursor result and failed to get session for printing",
				"userID", userID, "error", err, "result", result,
			)
		}
		return fmt.Errorf("unknown advance pull cursor result %d", result)
	}
}
