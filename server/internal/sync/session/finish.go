package session

import (
	"context"
	_ "embed"
	"errors"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
)

//go:embed scripts/check_finish_session.lua
var checkFinishSessionLua string
var checkFinishSessionScript = redis.NewScript(checkFinishSessionLua)

type FinishSyncLuaResult int64

const (
	FinishSyncLuaErr FinishSyncLuaResult = iota
	FinishSyncLuaOK
	FinishSyncLuaSessionNotFound
	FinishSyncLuaSessionIDMismatch
	FinishSyncLuaStateMismatch
)

//go:embed scripts/update_last_sync_time.sql
var updateLastSyncTimeSQL string

func (s *SessionStore) FinishSync(ctx context.Context, userID int64, sessionID string, finishTime int64) error {
	result, err := checkFinishSessionScript.Run(
		ctx,
		s.rdb,
		[]string{RdbSessionKey(userID)},
		sessionID,
		syncSessionTTLSecs,
	).Int64()

	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to finish sync", "userID", userID, "sessionID", sessionID, "error", err)
		return connect.NewError(connect.CodeInternal, nil)
	}

	switch FinishSyncLuaResult(result) {
	case FinishSyncLuaOK:
		_, err := s.db.ExecContext(ctx, updateLastSyncTimeSQL, finishTime, userID)
		if err != nil {
			s.logger.ErrorCtx(ctx, "failed to update last sync time", "userID", userID, "error", err)
			return connect.NewError(connect.CodeInternal, nil)
		}
		return s.releaseSyncSession(ctx, userID, sessionID)
	case FinishSyncLuaSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when finishing sync", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session not found when finishing sync")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	case FinishSyncLuaSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when finishing sync", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session id mismatch when finishing sync")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	case FinishSyncLuaStateMismatch:
		s.logger.ErrorCtx(ctx, "sync session state mismatch when finishing sync", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session state mismatch when finishing sync")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	default:
		s.logger.ErrorCtx(ctx, "unknown finish sync result", "result", result)
		return connect.NewError(connect.CodeInternal, nil)
	}
}

//go:embed scripts/release_sync_session.lua
var releaseSessionLua string
var releaseSessionScript = redis.NewScript(releaseSessionLua)

type ReleaseSessionLuaResult int64

const (
	ReleaseSessionLuaErr ReleaseSessionLuaResult = iota
	ReleaseSessionLuaOK
	ReleaseSessionLuaNotFound
	ReleaseSessionLuaIDMismatch
)

func (s *SessionStore) releaseSyncSession(ctx context.Context, userID int64, sessionID string) error {
	result, err := releaseSessionScript.Run(
		ctx,
		s.rdb,
		[]string{RdbSessionKey(userID)},
		sessionID,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to release sync session", "userID", userID, "sessionID", sessionID, "error", err)
		return connect.NewError(connect.CodeInternal, nil)
	}

	return s.handleReleaseResult(ctx, userID, sessionID, ReleaseSessionLuaResult(result))
}

func (s *SessionStore) handleReleaseResult(ctx context.Context, userID int64, sessionID string, result ReleaseSessionLuaResult) error {
	switch result {
	case ReleaseSessionLuaOK:
		return nil
	case ReleaseSessionLuaNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when releasing sync session", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session not found when releasing sync session")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	case ReleaseSessionLuaIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when releasing sync session", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session id mismatch when releasing sync session")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	default:
		s.logger.ErrorCtx(ctx, "unknown release sync session result", "result", result)
		return connect.NewError(connect.CodeInternal, nil)
	}
}
