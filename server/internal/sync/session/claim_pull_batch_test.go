package session

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestClaimPullBatchRedisError 测试 ClaimPullBatch 在 redis 连接错误时返回错误。
func TestClaimPullBatchRedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialerRetries: 1})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	store := NewSessionStore(nil, rdb, logging.NewServerLog())

	result, err := store.ClaimPullBatch(t.Context(), 10001, "session-id-1", 1)

	require.Error(t, err)
	require.Equal(t, ClaimPullBatchLuaErr, result.LuaResult)
}
