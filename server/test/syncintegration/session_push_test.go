package syncintegration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

func TestSessionStoreClaimPushBatch(t *testing.T) {
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

	// 推进 USN 3 changeCount，batch seq 为 1
	key := ss.RdbSessionKey(session.UserID)
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1, 3)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN, claimResult.AssignedStartUSN)
	gotBatchSeq, err := suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, session.ExpectedBatchSeq+1, gotBatchSeq)
	gotSyncCursorUSN, err := suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, session.SyncCursorUSN+3, gotSyncCursorUSN)

	// 测试 batch seq 不匹配
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1, 1)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaBatchSeqMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedStartUSN)

	// 测试 session id 不匹配
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, "other-session", 2, 1)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaSessionIDMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedStartUSN)

	// 测试正确的新 batch seq
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 2, 2)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN+3, claimResult.AssignedStartUSN)
	gotBatchSeq, err = suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, session.ExpectedBatchSeq+2, gotBatchSeq)
	gotSyncCursorUSN, err = suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, session.SyncCursorUSN+5, gotSyncCursorUSN)
}

// TestSessionStoreClaimPushBatchFromAwaitingPushOrFinish 测试从 AWAITING_PUSH_OR_FINISH 状态下 ClaimPushBatch
// 能否正确切换到 PUSHING 状态
func TestSessionStoreClaimPushBatchFromAwaitingPushOrFinish(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      10001,
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

	key := ss.RdbSessionKey(session.UserID)
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1, 2)
	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN, claimResult.AssignedStartUSN)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(ss.SyncSessionStatePushing), gotState)
}

// TestSessionStoreClaimPushBatchSessionNotFound 测试 ClaimPushBatch 在 session 不存在时返回对应结果码。
func TestSessionStoreClaimPushBatchSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	claimResult, err := store.ClaimPushBatch(t.Context(), 10001, "session-id-1", 1, 1)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaSessionNotFound, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedStartUSN)
}

// TestSessionStoreClaimPushBatchStateMismatch 测试 ClaimPushBatch 在 session state 不允许 Push 时返回对应结果码。
func TestSessionStoreClaimPushBatchStateMismatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		PullEntityQueue:             "4,6",
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1, 1)

	require.NoError(t, err)
	require.Equal(t, ss.ClaimPushBatchLuaStateMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedStartUSN)
}

// TestSessionStoreMarkPushFinished 测试 MarkPushFinished 将 session state 改为 AWAITING_FINISH
func TestSessionStoreMarkPushFinished(t *testing.T) {
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

	key := ss.RdbSessionKey(session.UserID)
	err = store.MarkPushFinished(ctx, session.UserID, session.SessionID)
	require.NoError(t, err)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(ss.SyncSessionStateAwaitingFinish), gotState)

	err = store.MarkPushFinished(ctx, session.UserID, "other-session")
	require.Error(t, err)
}

// TestSessionStoreMarkPushFinishedSessionNotFound
// 测试 MarkPushFinished 在 session 不存在时返回错误
func TestSessionStoreMarkPushFinishedSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())

	err := store.MarkPushFinished(t.Context(), 10001, "session-id-1")

	require.Error(t, err)
}
