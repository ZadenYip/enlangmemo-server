package syncintegration

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

// TestSessionStoreGetSessionSuccess 测试 GetSession 方法在有实际数据的时候，提取出的结果是否正确
func TestSessionStoreGetSessionSuccess(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	want := ss.SyncSession{
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            2,
		SyncCursorUSN:               12,
		PullEntityQueue:             "4,6",
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 18,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, want)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	got, err := store.GetSession(ctx, want.UserID)

	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestSessionStoreGetSessionNotFound 测试 GetSession 方法在没有数据的时应该返回错误
func TestSessionStoreGetSessionNotFound(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	userID := int64(10001)

	got, err := store.GetSession(t.Context(), userID)

	require.Error(t, err)
	require.Zero(t, got)
}

// TestSessionStoreGetSessionHGetAllError 测试 GetSession 方法在 HGetAll 返回错误时应该返回错误
func TestSessionStoreGetSessionHGetAllError(t *testing.T) {
	resetEnv(t)
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := store.GetSession(canceledCtx, 10001)

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, got)
}

// TestSessionStoreGetSessionScanError 测试 GetSession 方法在 Scan 转换 state 发现不是数字时应该返回错误
func TestSessionStoreGetSessionScanError(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := ss.NewSessionStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	userID := int64(10001)
	key := ss.RdbSessionKey(userID)

	_, err := suite.Env.RDB.HSet(ctx, key, map[string]any{
		"user_id":                             strconv.FormatInt(userID, 10),
		"state":                               "not-a-number",
		"expected_batch_seq":                  "1",
		"sync_cursor_usn":                     "12",
		"pull_entity_queue":                   "4,6",
		"session_id":                          "session-id-1",
		"client_sync_cursor_usn_at_handshake": "3",
		"server_sync_cursor_usn_at_handshake": "18",
		"device_id":                           "device-1",
	}).Result()
	require.NoError(t, err)

	got, err := store.GetSession(ctx, userID)

	require.Error(t, err)
	require.Zero(t, got)
}
