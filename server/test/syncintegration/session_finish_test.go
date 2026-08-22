package syncintegration

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

// TestSessionStoreFinishSyncSuccessReleasesSession
// 测试 FinishSync 成功后释放 session
func TestSessionStoreFinishSyncSuccessReleasesSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	userID := int64(10001)
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

	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(userID)).Result()
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

	err := store.FinishSync(t.Context(), 10001, "session-id-1", 1_800_000_000_000)

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
		UserID:                      10001,
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

	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(session.UserID)).Result()
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
		UserID:                      10001,
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

	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(session.UserID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

// TestSessionStoreFinishSyncAllowedStates
// 测试 FinishSync 在允许的 state 状态下能否成功 finish sync
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
			userID := int64(10001)
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

func insertFinishTestCol(t *testing.T, userID int64, lastSyncTime int64) {
	t.Helper()
	collectionID, err := uuid.NewV7()
	require.NoError(t, err)
	now := int64(1_700_000_000_000)

	_, err = suite.Env.DB.ExecContext(
		t.Context(),
		`INSERT INTO collections (
			user_id, id, usn, sqlite_schema_version, last_sync_time, sync_cursor_usn,
			created_at, updated_at, config
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, JSON_OBJECT())`,
		userID,
		collectionID[:],
		12,
		1,
		lastSyncTime,
		13,
		now,
		now,
	)
	require.NoError(t, err)
}
