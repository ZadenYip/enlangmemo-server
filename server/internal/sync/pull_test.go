package sync

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func TestClaimPullBatchMapsFailedPreconditionResults(t *testing.T) {
	tests := []struct {
		name        string
		claimResult ss.ClaimPullBatchLuaResult
	}{
		{
			name:        "missing session",
			claimResult: ss.ClaimPullBatchLuaSessionNotFound,
		},
		{
			name:        "session id mismatch",
			claimResult: ss.ClaimPullBatchLuaSessionIDMismatch,
		},
		{
			name:        "batch seq mismatch",
			claimResult: ss.ClaimPullBatchLuaBatchSeqMismatch,
		},
		{
			name:        "state mismatch",
			claimResult: ss.ClaimPullBatchLuaStateMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := int64(10001)
			ctx := context.WithValue(context.Background(), "userID", userID)
			req := &syncv1.PullRequest{SessionId: "session-id-1", BatchSeq: 1}
			sessionStore := &fakeSessionStore{}
			sessionStore.On("ClaimPullBatch", mock.Anything, userID, req.GetSessionId(), req.GetBatchSeq()).Return(
				ss.ClaimPullBatchResult{LuaResult: tt.claimResult},
				nil,
			)
			handler := &SyncHandler{
				sessionStore: sessionStore,
				logger:       logging.NewServerLog(),
			}

			result, err := handler.claimPullBatch(ctx, req, userID)

			require.Error(t, err)
			require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
			require.Zero(t, result)
			sessionStore.AssertExpectations(t)
		})
	}
}

// TestClaimPullBatchStoreError 测试 claimPullBatch 在 sessionStore 遇到
// redis 错误时返回 connect.CodeInternal
func TestClaimPullBatchStoreError(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0", DialerRetries: 1})
	t.Cleanup(func() {
		require.NoError(t, rdb.Close())
	})
	userID := int64(10001)
	ctx := context.WithValue(context.Background(), "userID", userID)
	store := ss.NewSessionStore(nil, rdb, logging.NewServerLog())
	handler := &SyncHandler{
		sessionStore: store,
		logger:       logging.NewServerLog(),
	}

	result, err := handler.claimPullBatch(ctx, &syncv1.PullRequest{SessionId: "session-id-1", BatchSeq: 1}, userID)

	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	require.Zero(t, result)
}

// TestPullReturnsClaimPullBatchError 测试 Pull 在 claimPullBatch 返回错误时直接返回该错误
func TestPullReturnsClaimPullBatchError(t *testing.T) {
	userID := int64(10001)
	ctx := context.WithValue(context.Background(), "userID", userID)
	req := connect.NewRequest(&syncv1.PullRequest{SessionId: "session-id-1", BatchSeq: 2})
	sessionStore := &fakeSessionStore{}
	sessionStore.On("ClaimPullBatch", mock.Anything, userID, req.Msg.GetSessionId(), req.Msg.GetBatchSeq()).Return(
		ss.ClaimPullBatchResult{LuaResult: ss.ClaimPullBatchLuaBatchSeqMismatch},
		nil,
	)
	handler := &SyncHandler{
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Pull(ctx, req)

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	require.EqualError(t, err, "failed_precondition: sync batch seq mismatch")
	sessionStore.AssertExpectations(t)
}

func TestPullReturnsInternalWhenEntityQueueCannotParse(t *testing.T) {
	userID := int64(10001)
	ctx := context.WithValue(context.Background(), "userID", userID)
	req := connect.NewRequest(&syncv1.PullRequest{SessionId: "session-id-1", BatchSeq: 1})
	sessionStore := &fakeSessionStore{}
	sessionStore.On("ClaimPullBatch", mock.Anything, userID, req.Msg.GetSessionId(), req.Msg.GetBatchSeq()).Return(
		ss.ClaimPullBatchResult{
			LuaResult:                   ss.ClaimPullBatchLuaOK,
			SyncCursorUSN:               1,
			SrvSyncCursorUSNAtHandshake: 8,
			PullEntityQueue:             "not-an-entity-type",
		},
		nil,
	)
	handler := &SyncHandler{
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Pull(ctx, req)

	require.Nil(t, resp)
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	sessionStore.AssertExpectations(t)
}

func TestParseTypeQueueString(t *testing.T) {

	// 空字符串表示没有待拉取的实体类型
	typeQueue := ""
	queue, err := parseEntityQueue(typeQueue)
	require.NoError(t, err)
	require.Empty(t, queue)

	// 测试非整数值
	typeQueue = "1,not-an-integer,3"
	_, err = parseEntityQueue(typeQueue)
	require.Error(t, err)
}
