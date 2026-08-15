package session

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestCreateSessionRedisError 测试 CreateSession 在 redis 连接错误时返回错误
func TestCreateSessionRedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	store := NewSessionStore(nil, rdb, logging.NewServerLog())
	session := SyncSession{
		UserID:                      "10001",
		State:                       SyncSessionStatePushing,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	result, err := store.CreateSession(t.Context(), session)

	require.Error(t, err)
	require.Equal(t, CreateSessionErr, result)
}
