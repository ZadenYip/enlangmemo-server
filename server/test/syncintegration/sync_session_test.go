package syncintegration

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncstore "github.com/zadenyip/enlangmemo-server/internal/sync"
)

// 测试正常创建 session，并验证 redis 中的值是否正确
func TestSessionStoreCreateSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	result, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, result)

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
	require.Equal(t, syncstore.CreateSessionAlreadyExists, result)
}

func TestSessionStoreClaimPushBatch(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, createResult)

	key := "sync:" + session.UserID + ":sync_lock"
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, syncstore.ClaimPushBatchLuaOK, claimResult.LuaResult)
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
	require.Equal(t, syncstore.ClaimPushBatchLuaBatchSeqMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedUSN)

	// 测试 session id 不匹配
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, "other-session", 2)
	require.NoError(t, err)
	require.Equal(t, syncstore.ClaimPushBatchLuaSessionIDMismatch, claimResult.LuaResult)
	require.Zero(t, claimResult.AssignedUSN)

	// 测试正确的新 batch seq
	claimResult, err = store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 2)
	require.NoError(t, err)
	require.Equal(t, syncstore.ClaimPushBatchLuaOK, claimResult.LuaResult)
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
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStateAwaitingPushOrFinish,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, createResult)

	// 测试 AWAITING_PUSH_OR_FINISH 在 ClaimPushBatch 后能否正确切换到 PUSHING 状态
	key := "sync:" + session.UserID + ":sync_lock"
	claimResult, err := store.ClaimPushBatch(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, syncstore.ClaimPushBatchLuaOK, claimResult.LuaResult)
	require.Equal(t, session.SyncCursorUSN, claimResult.AssignedUSN)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(syncstore.SyncSessionStatePushing), gotState)
}

// TestSessionStoreMarkPushFinished 测试 MarkPushFinished 将 session state 改为 AWAITING_FINISH
func TestSessionStoreMarkPushFinished(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, createResult)

	key := "sync:" + session.UserID + ":sync_lock"
	err = store.MarkPushFinished(ctx, session.UserID, session.SessionID)
	require.NoError(t, err)

	gotState, err := suite.Env.RDB.HGet(ctx, key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(syncstore.SyncSessionStateAwaitingFinish), gotState)

	err = store.MarkPushFinished(ctx, session.UserID, "other-session")
	require.Error(t, err)
}
