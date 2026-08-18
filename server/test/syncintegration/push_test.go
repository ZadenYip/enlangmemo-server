package syncintegration

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestPushCollectionMissingPayload 测试在 Push collection 时，
// 如果 payload 为空，服务端会返回 InvalidArgument 错误
func TestPushCollectionMissingPayload(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "missing-payload")
	collectionID := uuid.Must(uuid.NewV7()).String()
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	handshakeResp := startPushSync(t, client, accessToken, collectionID)

	resp, err := client.Push(t.Context(), newAuthorizedRequest(&syncv1.PushRequest{
		SessionId: handshakeResp.GetSessionId(),
		BatchSeq:  1,
		Changes: []*syncv1.SyncChange{
			{
				EntityId:   collectionID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        -1,
			},
		},
		LastBatch: true,
	}, accessToken))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestPushCollectionSuccess 测试在 Push collection 时，
// 如果 payload 正确，服务端会返回成功，并且数据库中会有对应的 collection 和 sync_unit 记录
func TestPushCollectionSuccess(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "push-collection")
	collectionID := uuid.Must(uuid.NewV7()).String()
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	handshakeResp := startPushSync(t, client, accessToken, collectionID)
	collection := &syncv1.CollectionPayload{
		SqliteSchemaVersion: 15,
		CreatedAt:           1_700_000_000_000,
		UpdatedAt:           1_700_000_000_100,
		ConfigJson:          `{"sync":"ok"}`,
	}

	resp, err := client.Push(t.Context(), newAuthorizedRequest(&syncv1.PushRequest{
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
	require.Equal(t, int32(1), resp.Msg.BatchSeq)
	require.Len(t, resp.Msg.Changes, 1)
	assignedChange := resp.Msg.Changes[0]
	require.Equal(t, collectionID, assignedChange.EntityId)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_COLLECTION, assignedChange.EntityType)
	require.Equal(t, syncv1.ChangeOp_CHANGE_OP_ASSIGN_USN, assignedChange.Op)
	require.Equal(t, handshakeResp.ServerSyncCursorUsn, assignedChange.GetUsn())
	assignedUSN := assignedChange.GetUsn()

	var gotCollection struct {
		USN                 int64
		SQLiteSchemaVersion int32
		SyncCursorUSN       int64
		CreatedAt           int64
		UpdatedAt           int64
		ConfigSync          string
		IsDeleted           bool
	}
	collectionUUID := uuid.MustParse(collectionID)
	// TODO 未来写一个 helper 简化下面的查询和扫描操作
	err = suite.Env.DB.QueryRowContext(
		t.Context(),
		`SELECT usn, sqlite_schema_version, sync_cursor_usn, created_at, updated_at, JSON_UNQUOTE(JSON_EXTRACT(config, '$.sync')), is_deleted
		 FROM collections
		 WHERE user_id = ? AND id = ?`,
		userID,
		collectionUUID[:],
	).Scan(
		&gotCollection.USN,
		&gotCollection.SQLiteSchemaVersion,
		&gotCollection.SyncCursorUSN,
		&gotCollection.CreatedAt,
		&gotCollection.UpdatedAt,
		&gotCollection.ConfigSync,
		&gotCollection.IsDeleted,
	)
	require.NoError(t, err)
	require.Equal(t, assignedUSN, gotCollection.USN)
	require.Equal(t, collection.SqliteSchemaVersion, gotCollection.SQLiteSchemaVersion)
	require.Equal(t, assignedUSN+1, gotCollection.SyncCursorUSN)
	require.Equal(t, collection.CreatedAt, gotCollection.CreatedAt)
	require.Equal(t, collection.UpdatedAt, gotCollection.UpdatedAt)
	require.Equal(t, "ok", gotCollection.ConfigSync)
	require.False(t, gotCollection.IsDeleted)

	var gotSyncUnit struct {
		USN        int64
		EntityType int32
		Op         int32
		UpdatedAt  int64
	}
	err = suite.Env.DB.QueryRowContext(
		t.Context(),
		`SELECT usn, entity_type, op, updated_at
		 FROM sync_units
		 WHERE user_id = ? AND entity_id = ?`,
		userID,
		collectionUUID[:],
	).Scan(&gotSyncUnit.USN, &gotSyncUnit.EntityType, &gotSyncUnit.Op, &gotSyncUnit.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, assignedUSN, gotSyncUnit.USN)
	require.Equal(t, int32(syncv1.EntityType_ENTITY_TYPE_COLLECTION), gotSyncUnit.EntityType)
	require.Equal(t, int32(syncv1.ChangeOp_CHANGE_OP_UPSERT), gotSyncUnit.Op)
	require.Equal(t, collection.UpdatedAt, gotSyncUnit.UpdatedAt)
}
