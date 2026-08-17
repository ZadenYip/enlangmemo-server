package syncintegration

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-server/test/testenv"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
	"github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1/syncv1connect"
)

func newSyncTestClient() syncv1connect.SyncServiceClient {
	return testenv.ConnectRPCClient(suite, syncv1connect.NewSyncServiceClient)
}

func newSyncTestAccessToken(t *testing.T, userID int64) string {
	t.Helper()
	store := oauth.NewOAStore(suite.Env.DB, suite.Env.RDB, logging.NewServerLog())
	accessToken, err := store.GenAccessToken(t.Context(), "sync-integration-client", userID)
	require.NoError(t, err)
	return accessToken
}

// newAuthorizedRequest 创建一个带有 Authorization header 的 ConnectRPC 请求
func newAuthorizedRequest[T any](msg *T, accessToken string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+accessToken)
	return req
}

func sendHandshake(t *testing.T, client syncv1connect.SyncServiceClient, accessToken string, req *syncv1.HandshakeRequest) *syncv1.HandshakeResponse {
	t.Helper()
	resp, err := client.Handshake(t.Context(), newAuthorizedRequest(req, accessToken))
	require.NoError(t, err)
	return resp.Msg
}

func startPushSync(t *testing.T, client syncv1connect.SyncServiceClient, accessToken, collectionID string) *syncv1.HandshakeResponse {
	t.Helper()
	resp := sendHandshake(t, client, accessToken, &syncv1.HandshakeRequest{
		DeviceId:            uuid.Must(uuid.NewV7()).String(),
		DeviceName:          "integration-test-device",
		CollectionId:        collectionID,
		ClientSyncCursorUsn: 1,
		ProtocolVersion:     1,
		DbSchemaVersion:     1,
		ClientNow:           time.Now().UnixMilli(),
		HasLocalChanges:     true,
	})
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES, resp.Status)
	require.NotNil(t, resp.SessionId)
	require.NotEmpty(t, resp.GetSessionId())
	require.Equal(t, int64(1), resp.ServerSyncCursorUsn)
	require.Zero(t, resp.ServerLastSyncTime)
	return resp
}
