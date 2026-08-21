package session

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestFinishSyncRedisError 测试 FinishSync 在 redis 连接错误时返回错误
func TestFinishSyncRedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialerRetries: 1})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	store := NewSessionStore(nil, rdb, logging.NewServerLog())

	err := store.FinishSync(t.Context(), 10001, "session-id-1", 1_800_000_000_000)

	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// TestReleaseSyncSessionRedisError 测试 releaseSyncSession 在 redis 脚本执行失败时返回内部错误。
func TestReleaseSyncSessionRedisError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialerRetries: 1})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	store := NewSessionStore(nil, rdb, logging.NewServerLog())

	err := store.releaseSyncSession(t.Context(), 10001, "session-id-1")

	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestHandleReleaseSessionResult(t *testing.T) {
	tests := []struct {
		name        string
		result      ReleaseSessionLuaResult
		wantErr     bool
		wantErrCode connect.Code
	}{
		{
			name:   "ok",
			result: ReleaseSessionLuaOK,
		},
		{
			name:        "session not found",
			result:      ReleaseSessionLuaNotFound,
			wantErr:     true,
			wantErrCode: connect.CodeFailedPrecondition,
		},
		{
			name:        "session id mismatch",
			result:      ReleaseSessionLuaIDMismatch,
			wantErr:     true,
			wantErrCode: connect.CodeFailedPrecondition,
		},
		{
			name:        "unknown result",
			result:      ReleaseSessionLuaResult(99),
			wantErr:     true,
			wantErrCode: connect.CodeInternal,
		},
	}

	store := &SessionStore{logger: logging.NewServerLog()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.handleReleaseResult(t.Context(), 10001, "session-id-1", tt.result)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrCode, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
		})
	}
}
