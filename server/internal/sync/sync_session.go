package sync

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"connectrpc.com/connect"
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

type ClaimPushBatchResult struct {
	LuaResult   ClaimPushBatchLuaResult
	AssignedUSN int64
}

type ClaimPushBatchLuaResult int64

const (
	ClaimPushBatchLuaErr ClaimPushBatchLuaResult = iota
	ClaimPushBatchLuaOK
	ClaimPushBatchLuaSessionNotFound
	ClaimPushBatchLuaSessionIDMismatch
	ClaimPushBatchLuaBatchSeqMismatch
	ClaimPushBatchLuaStateMismatch
)

type MarkPushFinishedResult int64

const (
	MarkPushFinishedErr MarkPushFinishedResult = iota
	MarkPushFinishedOK
	MarkPushFinishedSessionNotFound
	MarkPushFinishedSessionIDMismatch
)

type CreateSessionResult int64

const (
	CreateSessionErr CreateSessionResult = iota
	CreateSessionAlreadyExists
	CreateSessionCreated
)

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

//go:embed scripts/create_session.lua
var createSessionLua string
var createSessionScript = redis.NewScript(createSessionLua)

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

//go:embed scripts/claim_push_batch.lua
var claimPushBatchLua string
var claimPushBatchScript = redis.NewScript(claimPushBatchLua)

func (s *SessionStore) ClaimPushBatch(ctx context.Context, userID, sessionID string, curBatchSeq int64) (ClaimPushBatchResult, error) {
	rawResult, err := claimPushBatchScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		curBatchSeq,
		int64(syncSessionTTLSecs),
	).Int64Slice()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to claim push batch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq, "error", err)
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, err
	}

	if len(rawResult) != 2 {
		s.logger.ErrorCtx(ctx, "invalid claim push batch result", "result", rawResult)
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, fmt.Errorf("invalid claim push batch result: %v", rawResult)
	}

	result := ClaimPushBatchResult{
		LuaResult:   ClaimPushBatchLuaResult(rawResult[0]),
		AssignedUSN: rawResult[1],
	}

	switch result.LuaResult {
	case ClaimPushBatchLuaOK:
		return result, nil
	case ClaimPushBatchLuaSessionNotFound:
		s.logger.InfoCtx(ctx, "sync session not found", "userID", userID)
		return result, nil
	case ClaimPushBatchLuaSessionIDMismatch:
		s.logger.InfoCtx(ctx, "sync session id mismatch", "userID", userID, "sessionID", sessionID)
		return result, nil
	case ClaimPushBatchLuaBatchSeqMismatch:
		s.logger.InfoCtx(ctx, "sync batch seq mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	case ClaimPushBatchLuaStateMismatch:
		s.logger.InfoCtx(ctx, "sync session state mismatch", "userID", userID, "sessionID", sessionID, "currentBatchSeq", curBatchSeq)
		return result, nil
	default:
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown claim push batch result", "result", result.LuaResult, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown claim push batch result and failed to get session for printing", "userID", userID, "error", err)
		}
		return ClaimPushBatchResult{LuaResult: ClaimPushBatchLuaErr}, fmt.Errorf("unknown claim push batch result: %d", result.LuaResult)
	}
}

//go:embed scripts/mark_push_finished.lua
var markPushFinishedLua string
var markPushFinishedScript = redis.NewScript(markPushFinishedLua)

func (s *SessionStore) MarkPushFinished(ctx context.Context, userID, sessionID string) error {
	result, err := markPushFinishedScript.Run(
		ctx,
		s.rdb,
		[]string{rdbSessionKey(userID)},
		sessionID,
		int64(syncSessionTTLSecs),
	).Int64()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to mark push finished", "userID", userID, "sessionID", sessionID, "error", err)
		return err
	}

	switch MarkPushFinishedResult(result) {
	case MarkPushFinishedOK:
		return nil
	case MarkPushFinishedSessionNotFound:
		s.logger.ErrorCtx(ctx, "sync session not found when marking push finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session not found when marking push finished")
	case MarkPushFinishedSessionIDMismatch:
		s.logger.ErrorCtx(ctx, "sync session id mismatch when marking push finished", "userID", userID, "sessionID", sessionID)
		return errors.New("sync session id mismatch when marking push finished")
	default:
		s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result)
		if session, err := s.GetSession(ctx, userID); err == nil {
			s.logger.ErrorCtx(ctx, "unknown mark push finished result", "result", result, "session", session)
		} else {
			s.logger.ErrorCtx(ctx, "unknown mark push finished result and failed to get session for printing", "userID", userID, "error", err)
		}
		return fmt.Errorf("unknown mark push finished result: %d", result)
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
