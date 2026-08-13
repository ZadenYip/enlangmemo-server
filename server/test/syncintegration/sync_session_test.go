package syncintegration

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncstore "github.com/zadenyip/enlangmemo-server/internal/sync"
)

func syncTestSession() syncstore.SyncSession {
	return syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}
}

func TestSessionStoreCreateSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncTestSession()

	result, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, result)

	key := "sync:" + session.UserID + ":sync_lock"
	got, err := suite.Env.RDB.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"user_id":                             session.UserID,
		"state":                               strconv.FormatInt(int64(session.State), 10),
		"expected_batch_seq":                  "1",
		"sync_cursor_usn":                     "12",
		"session_id":                          session.SessionID,
		"client_sync_cursor_usn_at_handshake": "3",
		"server_sync_cursor_usn_at_handshake": "12",
		"device_id":                           session.DeviceID,
	}, got)

	ttl, err := suite.Env.RDB.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl.Seconds(), float64(60))

	result, err = store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionAlreadyExists, result)
}

func TestSessionStoreAdvSessionProgress(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncTestSession()

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, createResult)

	key := "sync:" + session.UserID + ":sync_lock"
	advanceResult, err := store.AdvSessionProgress(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, syncstore.AdvSessionLuaOK, advanceResult.LuaResult)
	require.Equal(t, int64(13), advanceResult.SyncCursorUSN)
	gotBatchSeq, err := suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(2), gotBatchSeq)
	gotSyncCursorUSN, err := suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(13), gotSyncCursorUSN)

	// 测试 batch seq 不匹配
	advanceResult, err = store.AdvSessionProgress(ctx, session.UserID, session.SessionID, 1)
	require.NoError(t, err)
	require.Equal(t, syncstore.AdvSessionLuaBatchSeqMismatch, advanceResult.LuaResult)
	require.Zero(t, advanceResult.SyncCursorUSN)

	// 测试 session id 不匹配
	advanceResult, err = store.AdvSessionProgress(ctx, session.UserID, "other-session", 2)
	require.NoError(t, err)
	require.Equal(t, syncstore.AdvSessionLuaSessionIDMismatch, advanceResult.LuaResult)
	require.Zero(t, advanceResult.SyncCursorUSN)

	// 测试正确的新 batch seq
	advanceResult, err = store.AdvSessionProgress(ctx, session.UserID, session.SessionID, 2)
	require.NoError(t, err)
	require.Equal(t, syncstore.AdvSessionLuaOK, advanceResult.LuaResult)
	require.Equal(t, int64(14), advanceResult.SyncCursorUSN)
	gotBatchSeq, err = suite.Env.RDB.HGet(ctx, key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(3), gotBatchSeq)
	gotSyncCursorUSN, err = suite.Env.RDB.HGet(ctx, key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(14), gotSyncCursorUSN)
}
