package session

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestMarkPushFinishedRedisError 测试 MarkPushFinished 在 redis 连接错误时返回错误
func TestClaimPushBatchRedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	store := NewSessionStore(nil, rdb, logging.NewServerLog())

	result, err := store.ClaimPushBatch(t.Context(), 10001, "session-id-1", 1)

	require.Error(t, err)
	require.Equal(t, ClaimPushBatchLuaErr, result.LuaResult)
	require.Zero(t, result.AssignedUSN)
}
