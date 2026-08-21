package sync

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestFinishWithoutUserID 覆盖 FinishSync 方法在没有 userID 是否报错
func TestFinishWithoutUserID(t *testing.T) {
	store := newFakeSessionStore(t, ss.CreateSessionCreated)
	syncHandler := NewSyncHandler(nil, nil, nil, nil, store)
	finReq := &syncv1.FinishSyncRequest{
		SessionId: "test-session-id",
	}
	req := connect.NewRequest(finReq)
	resp, err := syncHandler.FinishSync(t.Context(), req)
	require.Nil(t, resp, "expected nil response for FinishSync without userID")
	cntErr, ok := err.(*connect.Error)
	require.True(t, ok, "expected connect.Error")
	require.Equal(t, connect.CodeInternal, cntErr.Code(), "expected internal error code for FinishSync without userID")

	// store 不应该被调用，因为没有 userID
	require.Empty(t, store.Calls, "session store should not be called when userID is missing")
}
