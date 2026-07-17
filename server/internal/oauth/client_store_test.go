package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

func newFailingRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
}

func TestGetClientInfoInvalidUUIDReturnsNotFound(t *testing.T) {
	t.Helper()

	store := &OAStore{
		// 这里只需要一个会 miss/fail 的 Redis client，让代码继续走到 UUID 解析分支即可。
		rdb:    newFailingRedisClient(),
		logger: logging.NewServerLog(),
	}
	defer func() {
		_ = store.rdb.Close()
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	clientInfo, err := store.GetClientInfo(ctx, "not-a-uuid")

	require.ErrorIs(t, err, errOAClientNotFound)
	require.Equal(t, OAClientInfo{}, clientInfo)
}

func TestGetCachedClientInfoRedisErrorReturnsFalse(t *testing.T) {
	store := &OAStore{
		rdb:    newFailingRedisClient(),
		logger: logging.NewServerLog(),
	}
	defer func() {
		_ = store.rdb.Close()
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	clientInfo, ok := store.getCachedClientInfo(ctx, "client-id", "oauth:client:client-id")

	require.False(t, ok)
	require.Equal(t, OAClientInfo{}, clientInfo)
}

func TestCacheClientInfoRedisSetFailureDoesNotPanic(t *testing.T) {
	store := &OAStore{
		rdb:    newFailingRedisClient(),
		logger: logging.NewServerLog(),
	}
	defer func() {
		_ = store.rdb.Close()
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	require.NotPanics(t, func() {
		store.cacheClientInfo(ctx, "oauth:client:test", OAClientInfo{
			ClientID:    "00000000-0000-0000-0000-000000000001",
			RedirectURI: "https://client.example/callback",
		})
	})
}
