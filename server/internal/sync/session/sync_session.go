package session

import (
	"context"
	"database/sql"
	_ "embed"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

const (
	// syncSessionTTLSeconds
	syncSessionTTLSecs = 60
)

type SyncSession struct {
	UserID int64 `redis:"user_id"`

	// State 表示当前 session 所处的同步阶段
	State SessionState `redis:"state"`

	// ExpectedBatchSeq 是当前 State 下期望的下一个 batch 序号
	//
	// 进入 Pulling、Pushing、UploadingAll 或 AwaitingPushOrFinish 状态时重置为 1
	ExpectedBatchSeq int64 `redis:"expected_batch_seq"`

	// SyncCursorUSN 是当前 session 已处理到的 USN 上界 / 下一次处理起点。
	//
	// 在 PULLING 状态下，该字段复用为 PullEntityQueue[0] 对应实体类型内的拉取游标。
	// 当前实体类型拉完后会重置为 CliSyncCursorUSNAtHandshake，全部实体类型拉完后会推进到
	// SrvSyncCursorUSNAtHandshake
	SyncCursorUSN int64 `redis:"sync_cursor_usn"`

	// PullEntityQueue 是 PULLING 状态下剩余待拉取的实体类型队列，格式如 "1,2,4,6"。
	// 该字段是 Redis session 内部临时字段，Pull 完成后会删除。
	PullEntityQueue string `redis:"pull_entity_queue"`

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
	GetSession(ctx context.Context, userID int64) (SyncSession, error)
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
	ClaimPushBatch(ctx context.Context, userID int64, sessionID string, curBatchSeq int32) (ClaimPushBatchResult, error)

	// MarkPushFinished 在最后一个 Push batch 落库成功后，将 session state 改为 AWAITING_FINISH
	MarkPushFinished(ctx context.Context, userID int64, sessionID string) error

	// ClaimPullBatch 校验 Pull session 和 batch，并返回当前 Pull 游标与剩余实体类型队列
	// 如果校验失败则返回 (ClaimPullBatchResult, nil)，如果是内部错误则返回 (ClaimPullBatchResult{}, error)
	ClaimPullBatch(ctx context.Context, userID int64, sessionID string, curBatchSeq int32) (ClaimPullBatchResult, error)

	// AdvancePullCursor 在 Pull batch 处理完成后，更新 sync_cursor_usn
	AdvancePullCursor(ctx context.Context, userID int64, sessionID string, newSyncCursorUSN int64) error

	// MarkPullEntityFinished 在当前实体类型拉取完成后，
	// 更新剩余实体类型队列，并重置 sync_cursor_usn 为 client_sync_cursor_usn_at_handshake
	MarkPullEntityFinished(ctx context.Context, userID int64, sessionID, remainingPullEntityQueue string) error

	// MarkPullFinished 在最后一个 Pull batch 处理完成后，将 session state 改为 AWAITING_PUSH_OR_FINISH
	MarkPullFinished(ctx context.Context, userID int64, sessionID string) error

	// FinishSync 在客户端调用 FinishSync 后，删除当前用户的 SyncSession，表示本次同步完成
	// 会验证 sessionID 是否匹配以及 当前 state 可否 FinishSync，若不行则返回 connect.NewError 创建的错误
	FinishSync(ctx context.Context, userID int64, sessionID string, finishTime int64) error
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

func rdbSessionKey(userID int64) string {
	return "sync:" + strconv.FormatInt(userID, 10) + ":sync_lock"
}
