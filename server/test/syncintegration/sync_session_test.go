package syncintegration

import (
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

// 测试正常创建 session，并验证 redis 中的值是否正确
func TestSessionStoreCreateSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "session-user-1",
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	result, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, result)

	key := "sync:" + session.UserID + ":sync_lock"
	got, err := suite.Env.RDB.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"user_id":                             session.UserID,
		"state":                               strconv.FormatInt(int64(session.State), 10),
		"expected_batch_seq":                  strconv.FormatInt(session.ExpectedBatchSeq, 10),
		"sync_cursor_usn":                     strconv.FormatInt(session.SyncCursorUSN, 10),
		"session_id":                          session.SessionID,
		"client_sync_cursor_usn_at_handshake": strconv.FormatInt(session.CliSyncCursorUSNAtHandshake, 10),
		"server_sync_cursor_usn_at_handshake": strconv.FormatInt(session.SrvSyncCursorUSNAtHandshake, 10),
		"device_id":                           session.DeviceID,
	}, got)

	ttl, err := suite.Env.RDB.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl.Seconds(), float64(60))

	result, err = store.CreateSession(ctx, session)
	require.NoError(t, err)

	// 再次创建相同 session，应该返回 CreateSessionAlreadyExists
	require.Equal(t, ss.CreateSessionAlreadyExists, result)
}

func TestSessionStoreClaimPushBatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "session-user-1",
		State:                       ss.SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	key := "sync:" + session.UserID + ":sync_lock"
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN, claimResult.AssignedUSN)
	gotBatchSeq, err := suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, session.ExpectedBatchSeq+1, gotBatchSeq)
	gotSyncCursorUSN, err := suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, session.SyncCursorUSN+1, gotSyncCursorUSN)

	// 测试 batch seq 不匹配
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaBatchSeqMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedUSN)

	// 测试 session id 不匹配
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, "other-session", 2)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaSessionIDMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedUSN)

	// 测试正确的新 batch seq
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 2)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN+1, claimResult.AssignedUSN)
	gotBatchSeq, err = suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, session.ExpectedBatchSeq+2, gotBatchSeq)
	gotSyncCursorUSN, err = suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, session.SyncCursorUSN+2, gotSyncCursorUSN)
}

// TestSessionStoreClaimPushBatchFromAwaitingPushOrFinish 测试从 AWAITING_PUSH_OR_FINISH 状态下 ClaimPushBatch
func TestSessionStoreClaimPushBatchFromAwaitingPushOrFinish(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "session-user-1",
		State:                       ss.SyncSessionStateAwaitingPushOrFinish,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	// 测试 AWAITING_PUSH_OR_FINISH 在 ClaimPushBatch 后能否正确切换到 PUSHING 状态
	key := "sync:" + session.UserID + ":sync_lock"
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN, claimResult.AssignedUSN)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(ss.SyncSessionStatePushing), gotState)
}

// TestSessionStoreMarkPushFinished 测试 MarkPushFinished 将 session state 改为 AWAITING_FINISH
func TestSessionStoreMarkPushFinished(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "session-user-1",
		State:                       ss.SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	key := "sync:" + session.UserID + ":sync_lock"
	err = store.MarkPushFinished(ctx, session.UserID, session.SessionID)
	require.NoError(t, err)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(ss.SyncSessionStateAwaitingFinish), gotState)

	err = store.MarkPushFinished(ctx, session.UserID, "other-session")
	require.Error(t, err)
}

// TestSessionStoreFinishSyncSuccessReleasesSession
// 测试 FinishSync 成功后释放 session
func TestSessionStoreFinishSyncSuccessReleasesSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	userID := "10001"
	finishTime := int64(1_800_000_000_000)
	session := ss.SyncSession{
		UserID:                      userID,
		State:                       ss.SyncSessionStateAwaitingFinish,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}
	insertFinishTestCol(t, userID, 123)

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.FinishSync(ctx, userID, session.SessionID, finishTime)
	require.NoError(t, err)

	exists, err := suite.Env.RDB.Exists(ctx, "sync:"+userID+":sync_lock").Result()
	require.NoError(t, err)
	require.Zero(t, exists)

	var gotLastSyncTime int64
	err = suite.Env.DB.QueryRowContext(ctx, `SELECT last_sync_time FROM collections WHERE user_id = ?`, userID).Scan(&gotLastSyncTime)
	require.NoError(t, err)
	require.Equal(t, finishTime, gotLastSyncTime)
}

// TestSessionStoreFinishSyncSessionNotFound
// 测试 FinishSync 在 session 不存在时返回错误
func TestSessionStoreFinishSyncSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	err := store.FinishSync(t.Context(), "10001", "session-id-1", 1_800_000_000_000)

	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestSessionStoreFinishSyncStateMismatch
// 测试 FinishSync 在 session id 不匹配时返回错误，并且不会删除 redis 中的 sync_lock
func TestSessionStoreFinishSyncSessionIDMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "10001",
		State:                       ss.SyncSessionStateAwaitingFinish,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.FinishSync(ctx, session.UserID, "other-session", 1_800_000_000_000)
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	exists, err := suite.Env.RDB.Exists(ctx, "sync:"+session.UserID+":sync_lock").Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

// TestSessionStoreFinishSyncStateMismatch
// 测试 FinishSync 当前 state 状态不允许进行 Finish 的情况下返回错误
func TestSessionStoreFinishSyncStateMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      "10001",
		State:                       ss.SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.FinishSync(ctx, session.UserID, session.SessionID, 1_800_000_000_000)
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	exists, err := suite.Env.RDB.Exists(ctx, "sync:"+session.UserID+":sync_lock").Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

func TestSessionStoreFinishSyncAllowedStates(t *testing.T) {
	tests := []struct {
		name  string
		state ss.SessionState
	}{
		{name: "awaiting push or finish", state: ss.SyncSessionStateAwaitingPushOrFinish},
		{name: "awaiting finish", state: ss.SyncSessionStateAwaitingFinish},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnv(t)
			ctx := t.Context()
			store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
			userID := "10001"
			session := ss.SyncSession{
				UserID:                      userID,
				State:                       tt.state,
				ExpectedBatchSeq:            1,
				SyncCursorUSN:               12,
				SessionID:                   "session-id-1",
				CliSyncCursorUSNAtHandshake: 3,
				SrvSyncCursorUSNAtHandshake: 12,
				DeviceID:                    "device-1",
			}
			insertFinishTestCol(t, userID, 123)

			createResult, err := store.CreateSession(ctx, session)
			require.NoError(t, err)
			require.Equal(t, ss.CreateSessionCreated, createResult)

			err = store.FinishSync(ctx, userID, session.SessionID, 1_800_000_000_000)
			require.NoError(t, err)
		})
	}
}

func insertFinishTestCol(t *testing.T, userID string, lastSyncTime int64) {
	t.Helper()
	collectionID, err := uuid.NewV7()
	require.NoError(t, err)
	now := int64(1_700_000_000_000)

	_, err = suite.Env.DB.ExecContext(
		t.Context(),
		`INSERT INTO collections (
			user_id, id, usn, sqlite_schema_version, last_sync_time, sync_cursor_usn,
			created_at, updated_at, config, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, JSON_OBJECT(), ?)`,
		userID,
		collectionID[:],
		12,
		1,
		lastSyncTime,
		13,
		now,
		now,
		0,
	)
	require.NoError(t, err)
}
