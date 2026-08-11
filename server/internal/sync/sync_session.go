package sync

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// syncSessionTTLSeconds
const syncSessionTTLSecs = 60

type SyncSession struct {
	UserID string `redis:"user_id"`

	// State 表示当前 session 所处的同步阶段
	State SessionState `redis:"state"`

	// ExpectedBatchSeq 是当前 State 下期望的下一个 batch 序号
	//
	// 进入 Pulling、Pushing、UploadingAll 或 AwaitingPushOrFinish 状态时重置为 1
	ExpectedBatchSeq int64 `redis:"expected_batch_seq"`

	// SyncCursorUSN 是当前 session 已处理到的 USN 上界 / 下一次处理起点
	SyncCursorUSN int64 `redis:"sync_cursor_usn"`

	SessionID string `redis:"session_id"`

	CliSyncCursorUSNAtHandshake int64 `redis:"client_sync_cursor_usn_at_handshake"`
	SrvSyncCursorUSNAtHandshake int64 `redis:"server_sync_cursor_usn_at_handshake"`

	DeviceID string `redis:"device_id"`
}

type SessionState int

const (
	SyncSessionStateUnspecified SessionState = iota
	SyncSessionStatePulling
	SyncSessionStatePushing
	SyncSessionStateAwaitingPushOrFinish
	SyncSessionStateAwaitingFinish
	SyncSessionStateAwaitingUploadAllConfirm
	SyncSessionStateUploadingAll
)

type SessionStorer interface {
	// CreateSession 尝试创建一个新的 SyncSession
	//
	// CreateSessionResult 返回值含义如下
	// CreateSessionAlreadyExists 表示已经存在一个 session
	// CreateSessionCreated 表示创建成功
	//
	// 如果遇到错误，返回 CreateSessionErr，并且 err 不为 nil
	//
	// Session 超时时间为 60 秒
	CreateSession(ctx context.Context, session SyncSession) (CreateSessionResult, error)
	// GetSessionLock(userID string) (*SyncSession, error)
}

type CreateSessionResult int64

const (
	CreateSessionErr           CreateSessionResult = 0
	CreateSessionAlreadyExists CreateSessionResult = 1
	CreateSessionCreated       CreateSessionResult = 2
)

type SessionStore struct {
	rdb    *redis.Client
	logger logging.Logger
}

func NewSessionStore(rdb *redis.Client, logger logging.Logger) *SessionStore {
	return &SessionStore{
		rdb:    rdb,
		logger: logger,
	}
}

func rdbSessionKey(userID string) string {
	return "sync:" + userID + ":sync_lock"
}

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
		int64(syncSessionTTLSecs),
	}
}
