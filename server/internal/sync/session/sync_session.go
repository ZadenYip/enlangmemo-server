package session

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

const (
	// syncSessionTTLSeconds
	syncSessionTTLSecs = 60
)

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

// 注意此处变更值顺序记得更新对应的 LUA 脚本，因为 LUA 脚本中使用了数字表示状态，变更顺序会导致 LUA 脚本逻辑错误
// 最好不要变更顺序
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
	// GetSession 获取当前用户 SyncSession 的完整快照。
	// 如果 session 不存在或字段不完整，返回 error。
	GetSession(ctx context.Context, userID string) (SyncSession, error)
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
	// ClaimPushBatch 校验 Push session 和 batch，并分配 assigned_usn（可用的下个 sync_cursor_usn）
	ClaimPushBatch(ctx context.Context, userID, sessionID string, curBatchSeq int64) (ClaimPushBatchResult, error)
	// MarkPushFinished 在最后一个 Push batch 落库成功后，将 session state 改为 AWAITING_FINISH
	MarkPushFinished(ctx context.Context, userID, sessionID string) error
	// FinishSync 在客户端调用 FinishSync 后，删除当前用户的 SyncSession，表示本次同步完成
	// 会验证 sessionID 是否匹配以及 当前 state 可否 FinishSync，若不行则返回 connect.NewError 创建的错误
	FinishSync(ctx context.Context, userID, sessionID string, finishTime int64) error
}

type SessionStore struct {
	db     *sql.DB
	rdb    *redis.Client
	logger logging.Logger
}

func NewSessionStore(db *sql.DB, rdb *redis.Client, logger logging.Logger) *SessionStore {
	return &SessionStore{
		db:     db,
		rdb:    rdb,
		logger: logger,
	}
}

func rdbSessionKey(userID string) string {
	return "sync:" + userID + ":sync_lock"
}

// CreateSession 使用了 create_session.lua 脚本创建 SyncSession，保证原子性
func (s *SessionStore) GetSession(ctx context.Context, userID string) (SyncSession, error) {
	cmd := s.rdb.HGetAll(ctx, rdbSessionKey(userID))
	fields, err := cmd.Result()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get sync session", "userID", userID, "error", err)
		return SyncSession{}, err
	}
	if len(fields) == 0 {
		return SyncSession{}, fmt.Errorf("sync session not found for userID %s", userID)
	}

	var session SyncSession
	if err := cmd.Scan(&session); err != nil {
		s.logger.ErrorCtx(ctx, "failed to scan sync session", "userID", userID, "fields", fields, "error", err)
		return SyncSession{}, err
	}

	return session, nil
}
