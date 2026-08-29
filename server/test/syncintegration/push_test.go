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
	collectionUUID := uuid.Must(uuid.NewV7())
	collectionID := collectionUUID[:]
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
		FinishPush: false,
	}, accessToken))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestPushCollectionSuccess 测试在 Push collection 时，
// 如果 payload 正确，服务端会返回成功，并且数据库中会有对应的 collection 和 sync_unit 记录
func TestPushCollectionSuccess(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "push-collection")
	collectionUUID := uuid.Must(uuid.NewV7())
	collectionID := collectionUUID[:]
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
		FinishPush: false,
	}, accessToken))

	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.BatchSeq)
	require.Len(t, resp.Msg.Changes, 1)

	finishPushResp := sendFinishPush(t, client, accessToken, handshakeResp.GetSessionId(), 2)
	require.Equal(t, int32(2), finishPushResp.BatchSeq)
	require.Empty(t, finishPushResp.Changes)

	assignedChange := resp.Msg.Changes[0]
	require.Equal(t, collectionID, assignedChange.EntityId)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_COLLECTION, assignedChange.EntityType)
	require.Equal(t, syncv1.ChangeOp_CHANGE_OP_ASSIGN_USN, assignedChange.Op)
	require.Equal(t, handshakeResp.ServerSyncCursorUsn, assignedChange.GetUsn())
	assignedUSN := assignedChange.GetUsn()

	gotCol := getSyncTestCollection(t, userID, collectionID)
	require.Equal(t, collectionID, gotCol.ID)
	require.Equal(t, assignedUSN, gotCol.USN)
	require.Equal(t, collection.SqliteSchemaVersion, gotCol.SQLiteSchemaVersion)
	require.Equal(t, assignedUSN+1, gotCol.SyncCursorUSN)
	require.Equal(t, collection.CreatedAt, gotCol.CreatedAt)
	require.Equal(t, collection.UpdatedAt, gotCol.UpdatedAt)
	require.JSONEq(t, collection.ConfigJson, gotCol.ConfigJSON)
	require.False(t, gotCol.IsDeleted)

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
		collectionID,
	).Scan(&gotSyncUnit.USN, &gotSyncUnit.EntityType, &gotSyncUnit.Op, &gotSyncUnit.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, assignedUSN, gotSyncUnit.USN)
	require.Equal(t, int32(syncv1.EntityType_ENTITY_TYPE_COLLECTION), gotSyncUnit.EntityType)
	require.Equal(t, int32(syncv1.ChangeOp_CHANGE_OP_UPSERT), gotSyncUnit.Op)
	require.Equal(t, collection.UpdatedAt, gotSyncUnit.UpdatedAt)
}

// TestPushDeleteAlreadyDeletedDeckDoesNotCreateSyncUnit 测试删除已经被删除的实体时，
// 服务端会消费该 change 并分配 USN，但不会产生新的 sync_unit。
func TestPushDeleteAlreadyDeletedDeckDoesNotCreateSyncUnit(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "del-deck-noop")
	collectionID := insertSyncTestCollection(t, userID, syncTestCollectionRow{
		sqliteSchemaVersion: 15,
		lastSyncTime:        0,
		syncCursorUSN:       1,
	})

	// 这里插入标记删除的 deck
	deckUUID := uuid.Must(uuid.NewV7())
	deckID := deckUUID[:]
	oldDeckUSN := int64(1)
	oldDeckUpdatedAt := int64(1_700_000_000_000)
	deletedAt := int64(1_700_000_000_100)
	_, err := suite.Env.DB.ExecContext(
		t.Context(),
		`INSERT INTO decks (
			user_id, id, usn, name, updated_at,
			new_cards_per_day, new_learned_today, learned_today, reviewed_today,
			config, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		userID,
		deckID,
		oldDeckUSN,
		"deleted deck",
		oldDeckUpdatedAt,
		20,
		0,
		0,
		0,
		`{}`,
	)
	require.NoError(t, err)

	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	handshakeResp := startPushSync(t, client, accessToken, collectionID)
	resp, err := client.Push(t.Context(), newAuthorizedRequest(&syncv1.PushRequest{
		SessionId: handshakeResp.GetSessionId(),
		BatchSeq:  1,
		Changes: []*syncv1.SyncChange{
			{
				EntityId:   deckID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				Usn:        -1,
				DeletedAt:  &deletedAt,
			},
		},
	}, accessToken))

	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.BatchSeq)
	require.Len(t, resp.Msg.Changes, 1)
	assignedChange := resp.Msg.Changes[0]
	require.Equal(t, deckID, assignedChange.GetEntityId())
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_DECK, assignedChange.GetEntityType())
	require.Equal(t, syncv1.ChangeOp_CHANGE_OP_ASSIGN_USN, assignedChange.GetOp())
	require.Equal(t, handshakeResp.GetServerSyncCursorUsn(), assignedChange.GetUsn())

	var syncUnitCount int
	err = suite.Env.DB.QueryRowContext(
		t.Context(),
		`SELECT COUNT(*) FROM sync_units WHERE user_id = ? AND entity_id = ?`,
		userID,
		deckID,
	).Scan(&syncUnitCount)
	require.NoError(t, err)
	require.Zero(t, syncUnitCount)

	var gotDeckUSN, gotDeckUpdatedAt int64
	var gotIsDeleted bool
	err = suite.Env.DB.QueryRowContext(
		t.Context(),
		`SELECT usn, updated_at, is_deleted FROM decks WHERE user_id = ? AND id = ?`,
		userID,
		deckID,
	).Scan(&gotDeckUSN, &gotDeckUpdatedAt, &gotIsDeleted)
	require.NoError(t, err)
	require.Equal(t, oldDeckUSN, gotDeckUSN)
	require.Equal(t, oldDeckUpdatedAt, gotDeckUpdatedAt)
	// 本身就标记删除了
	require.True(t, gotIsDeleted)

	gotCol := getSyncTestCollection(t, userID, collectionID)
	require.Equal(t, assignedChange.GetUsn()+1, gotCol.SyncCursorUSN)
}
