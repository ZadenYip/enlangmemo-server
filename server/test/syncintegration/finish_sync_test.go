package syncintegration

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestFinishSyncSuccess 测试 FinishSync 成功的情况
func TestFinishSyncSuccess(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "finish-success")
	collectionUUID := uuid.Must(uuid.NewV7())
	collectionID := collectionUUID[:]
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	handshakeResp := startPushSync(t, client, accessToken, collectionID)
	collection := &syncv1.CollectionPayload{
		SqliteSchemaVersion: 15,
		CreatedAt:           1_700_000_000_000,
		UpdatedAt:           1_700_000_000_100,
		ConfigJson:          `{"sync":"finish"}`,
	}

	pushResp, err := client.Push(t.Context(), newAuthorizedRequest(&syncv1.PushRequest{
		SessionId: handshakeResp.GetSessionId(),
		BatchSeq:  1,
		Changes: []*syncv1.SyncChange{
			{
				EntityId:   collectionID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        -1,
				Payload:    &syncv1.SyncChange_Collection{Collection: collection},
			},
		},
		LastBatch: true,
	}, accessToken))
	require.NoError(t, err)
	require.Len(t, pushResp.Msg.Changes, 1)
	require.Equal(t, handshakeResp.ServerSyncCursorUsn, pushResp.Msg.Changes[0].GetUsn())

	finishResp, err := client.FinishSync(t.Context(), newAuthorizedRequest(&syncv1.FinishSyncRequest{
		SessionId: handshakeResp.GetSessionId(),
	}, accessToken))

	require.NoError(t, err)
	require.Positive(t, finishResp.Msg.ServerFinishedAt)

	exists, err := suite.Env.RDB.Exists(t.Context(), syncSessionTestKey(userID)).Result()
	require.NoError(t, err)
	require.Zero(t, exists)

	var gotLastSyncTime int64
	err = suite.Env.DB.QueryRowContext(t.Context(), `SELECT last_sync_time FROM collections WHERE user_id = ?`, userID).Scan(&gotLastSyncTime)
	require.NoError(t, err)
	require.Equal(t, finishResp.Msg.ServerFinishedAt, gotLastSyncTime)
}

// TestFinishSyncSessionNotFound 测试 FinishSync 在 session 不存在时返回错误
func TestFinishSyncSessionNotFound(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "finish-missing")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	missingSessionID := "11111111111111111111111111111111"

	resp, err := client.FinishSync(t.Context(), newAuthorizedRequest(&syncv1.FinishSyncRequest{
		SessionId: missingSessionID,
	}, accessToken))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestFinishSyncSessionIDMismatch 测试 FinishSync 在 session id 不匹配时返回错误，
// 并且不会删除 redis 中的 sync_lock
func TestFinishSyncSessionIDMismatch(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "finish-mismatch")
	collectionUUID := uuid.Must(uuid.NewV7())
	collectionID := collectionUUID[:]
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	handshakeResp := startPushSync(t, client, accessToken, collectionID)
	mismatchedSessionID := "22222222222222222222222222222222"
	require.NotEqual(t, handshakeResp.GetSessionId(), mismatchedSessionID)

	resp, err := client.FinishSync(t.Context(), newAuthorizedRequest(&syncv1.FinishSyncRequest{
		SessionId: mismatchedSessionID,
	}, accessToken))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	exists, err := suite.Env.RDB.Exists(t.Context(), syncSessionTestKey(userID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}
