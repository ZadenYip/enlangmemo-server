package syncintegration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

// TestSessionStoreClaimPullBatchSessionNotFound 测试 ClaimPullBatch 在 session 不存在时返回
// ClaimPullBatchLuaSessionNotFound
func TestSessionStoreClaimPullBatchSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	claimResult, err := store.ClaimPullBatch(t.Context(), 10001, "session-id-1", 1)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPullBatchLuaSessionNotFound, claimResult.LuaResult)
	require.Zero(t, claimResult.SyncCursorUSN)
	require.Zero(t, claimResult.SrvSyncCursorUSNAtHandshake)
	require.Empty(t, claimResult.PullEntityQueue)
}

// TestSessionStoreClaimPullBatchSessionIDMismatch 测试 ClaimPullBatch 在 session id 不匹配时返回
// ClaimPullBatchLuaSessionIDMismatch
func TestSessionStoreClaimPullBatchSessionIDMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := newClaimPullBatchTestSession()

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	claimResult, err := store.ClaimPullBatch(ctx, session.UserID, "other-session", 1)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPullBatchLuaSessionIDMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.SyncCursorUSN)
	require.Zero(t, claimResult.SrvSyncCursorUSNAtHandshake)
	require.Empty(t, claimResult.PullEntityQueue)
}

// TestSessionStoreClaimPullBatchBatchSeqMismatch 测试 ClaimPullBatch 在 batch seq 不匹配时返回
// ClaimPullBatchLuaBatchSeqMismatch
func TestSessionStoreClaimPullBatchBatchSeqMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := newClaimPullBatchTestSession()

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	claimResult, err := store.ClaimPullBatch(ctx, session.UserID, session.SessionID, 2)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPullBatchLuaBatchSeqMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.SyncCursorUSN)
	require.Zero(t, claimResult.SrvSyncCursorUSNAtHandshake)
	require.Empty(t, claimResult.PullEntityQueue)
}

// TestSessionStoreClaimPullBatchStateMismatch 测试 ClaimPullBatch 在 session state 不允许 Pull 时返回
// ClaimPullBatchLuaStateMismatch
func TestSessionStoreClaimPullBatchStateMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := newClaimPullBatchTestSession()

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)
	err = suite.Env.RDB.HSet(ctx, ss.RdbSessionKey(session.UserID), "state", int64(ss.SyncSessionStatePushing)).Err()
	require.NoError(t, err)

	claimResult, err := store.ClaimPullBatch(ctx, session.UserID, session.SessionID, 1)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPullBatchLuaStateMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.SyncCursorUSN)
	require.Zero(t, claimResult.SrvSyncCursorUSNAtHandshake)
	require.Empty(t, claimResult.PullEntityQueue)
}

func newClaimPullBatchTestSession() ss.SyncSession {
	return ss.SyncSession{
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               3,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
		PullEntityQueue:             "4,6",
	}
}

// TestSessionStoreMarkPullFinishedSessionNotFound
// 测试 MarkPullFinished 在 session 不存在时返回错误
func TestSessionStoreMarkPullFinishedSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	err := store.MarkPullFinished(t.Context(), 10001, "session-id-1")

	require.Error(t, err)
}

// TestSessionStoreMarkPullFinishedSessionIDMismatch
// 测试 MarkPullFinished 在 session id 不匹配时返回错误，并且 sync session 不被删除
func TestSessionStoreMarkPullFinishedSessionIDMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               3,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
		PullEntityQueue:             "4,6",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.MarkPullFinished(ctx, session.UserID, "other-session")

	require.Error(t, err)
	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(session.UserID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

// TestSessionStoreUpdatePullProgressSessionNotFound
// 测试 UpdatePullProgress 在 session 不存在时返回错误
func TestSessionStoreUpdatePullProgressSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	err := store.UpdatePullProgress(t.Context(), 10001, "session-id-1", "4,6", 7)

	require.Error(t, err)
}

// TestSessionStoreUpdatePullProgressSessionIDMismatch
// 测试 UpdatePullProgress 在 session id 不匹配时返回错误，并保留 sync_lock
func TestSessionStoreUpdatePullProgressSessionIDMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               3,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
		PullEntityQueue:             "4,6",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.UpdatePullProgress(ctx, session.UserID, "other-session", "6", 8)

	require.Error(t, err)
	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(session.UserID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}
