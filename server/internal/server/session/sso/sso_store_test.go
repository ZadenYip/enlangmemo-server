package sso

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisSSOStoreCreateSetNXError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{})
	defer func() {
		require.NoError(t, rdb.Close())
	}()
	store := &RedisSSOStore{Rdb: rdb}

	sessionID, err := store.Create(t.Context(), "user-id")

	require.Empty(t, sessionID)
	require.Error(t, err)
}
