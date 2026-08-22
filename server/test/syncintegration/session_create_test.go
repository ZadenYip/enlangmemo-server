package syncintegration

import (
	"strconv"
	"testing"

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
		UserID:                      10001,
		State:                       ss.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
		PullEntityQueue:             "1,2",
	}

	result, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, result)

	key := ss.RdbSessionKey(session.UserID)
	got, err := suite.Env.RDB.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"user_id":                             strconv.FormatInt(session.UserID, 10),
		"state":                               strconv.FormatInt(int64(session.State), 10),
		"expected_batch_seq":                  strconv.FormatInt(session.ExpectedBatchSeq, 10),
		"sync_cursor_usn":                     strconv.FormatInt(session.SyncCursorUSN, 10),
		"session_id":                          session.SessionID,
		"client_sync_cursor_usn_at_handshake": strconv.FormatInt(session.CliSyncCursorUSNAtHandshake, 10),
		"server_sync_cursor_usn_at_handshake": strconv.FormatInt(session.SrvSyncCursorUSNAtHandshake, 10),
		"device_id":                           session.DeviceID,
		"pull_entity_queue":                   session.PullEntityQueue,
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
