package session

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type CreateSessionResult int64

const (
	CreateSessionErr CreateSessionResult = iota
	CreateSessionAlreadyExists
	CreateSessionCreated
)

//go:embed scripts/create_session.lua
var createSessionLua string
var createSessionScript = redis.NewScript(createSessionLua)

// CreateSession 使用了 create_session.lua 脚本创建 SyncSession，保证原子性
func (s *SessionStore) CreateSession(ctx context.Context, session SyncSession) (CreateSessionResult, error) {
	result, err := createSessionScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(session.UserID)},
		sessionScriptArgs(session)...,
	).Int64()

	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to create session", "error", err)
		return CreateSessionErr, err
	}
	switch CreateSessionResult(result) {
	case CreateSessionAlreadyExists:
		s.logger.InfoCtx(ctx, "sync session already exists", "userID", session.UserID)
		return CreateSessionAlreadyExists, nil
	case CreateSessionCreated:
		s.logger.InfoCtx(ctx, "sync session created", "sessionID", session.SessionID)
		return CreateSessionCreated, nil
	default:
		s.logger.ErrorCtx(ctx, "unknown create session result", "result", result)
		return CreateSessionErr, fmt.Errorf("unknown create session result: %d", result)
	}
}

func sessionScriptArgs(session SyncSession) []any {
	return []any{
		session.UserID,
		int64(session.State),
		session.ExpectedBatchSeq,
		session.SyncCursorUSN,
		session.SessionID,
		session.CliSyncCursorUSNAtHandshake,
		session.SrvSyncCursorUSNAtHandshake,
		session.DeviceID,
		syncSessionTTLSecs,
	}
}
