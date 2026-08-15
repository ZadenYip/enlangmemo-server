package session

import (
	"context"
	_ "embed"
	"errors"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
)

//go:embed scripts/finish_sync.lua
var checkSyncFinishLua string
var checkSyncFinishScript = redis.NewScript(checkSyncFinishLua)

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

func (s *SessionStore) FinishSync(ctx context.Context, userID, sessionID string, finishTime int64) error {
	result, err := checkSyncFinishScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
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
var releaseSyncSessionLua string
var releaseSyncSessionScript = redis.NewScript(releaseSyncSessionLua)

func (s *SessionStore) releaseSyncSession(ctx context.Context, userID, sessionID string) error {
	result, err := releaseSyncSessionScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to release sync session", "userID", userID, "sessionID", sessionID, "error", err)
		return connect.NewError(connect.CodeInternal, nil)
	}

	switch FinishSyncLuaResult(result) {
	case FinishSyncLuaOK:
		return nil
	case FinishSyncLuaSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when releasing sync session", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session not found when releasing sync session")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	case FinishSyncLuaSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when releasing sync session", "userID", userID, "sessionID", sessionID)
		errInfo := errors.New("sync session id mismatch when releasing sync session")
		return connect.NewError(connect.CodeFailedPrecondition, errInfo)
	default:
		s.logger.ErrorCtx(ctx, "unknown release sync session result", "result", result)
		return connect.NewError(connect.CodeInternal, nil)
	}
}
