package syncintegration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
	"github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1/syncv1connect"
	"google.golang.org/protobuf/proto"
)

// TestPullAllEntityTypesThenFinishReleasesSession 测试在拉取所有实体类型后，调用 FinishSync 会释放 session
func TestPullAllEntityTypesThenFinishReleasesSession(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "fullpull")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	now := time.Now().UnixMilli()

	inserter := newPullTestInserter(t, userID, 1)
	colChange := inserter.InsertCollection(8)
	deckChange := inserter.InsertDeck(now + 2)
	noteTypeChange := inserter.InsertNoteType(now+3, `{"template":true}`)
	noteChange := inserter.InsertNote(now+4, `{"front":"server"}`)
	pcsNoteChange := inserter.InsertProcessingNote(now+5, `{"front":"processing"}`)
	cardChange := inserter.InsertCard(noteChange.entityID, now+6)
	reviewLogChange := inserter.InsertReviewLog(cardChange.entityID, now+7)
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 1, 8)

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.True(t, pullResp.LastBatch)
	require.Len(t, pullResp.Changes, 7)
	requirePulledChangesContainUpsert(t, pullResp.Changes, colChange.entityID, syncv1.EntityType_ENTITY_TYPE_COLLECTION)
	requirePulledChangesContainUpsert(t, pullResp.Changes, deckChange.entityID, syncv1.EntityType_ENTITY_TYPE_DECK)
	requirePulledChangesContainUpsert(t, pullResp.Changes, noteTypeChange.entityID, syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE)
	requirePulledChangesContainUpsert(t, pullResp.Changes, noteChange.entityID, syncv1.EntityType_ENTITY_TYPE_NOTE)
	requirePulledChangesContainUpsert(t, pullResp.Changes, pcsNoteChange.entityID, syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE)
	requirePulledChangesContainUpsert(t, pullResp.Changes, cardChange.entityID, syncv1.EntityType_ENTITY_TYPE_CARD)
	requirePulledChangesContainUpsert(t, pullResp.Changes, reviewLogChange.entityID, syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG)

	finishResp, err := client.FinishSync(t.Context(), newAuthorizedRequest(&syncv1.FinishSyncRequest{
		SessionId: hskResp.GetSessionId(),
	}, accessToken))
	require.NoError(t, err)
	require.Positive(t, finishResp.Msg.ServerFinishedAt)
	requireSessionReleased(t, userID)
}

// TestFullFlowPushPullDelete 测试两个客户端之间的完整推拉删除流程
// ClientA 推送 7 个实体类型的 upsert change，ClientB 拉取后再推送 3 个 delete change，ClientA 再拉取
func TestFullPushPullDeleteFlow(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "flowall")
	accessToken := newSyncTestAccessToken(t, userID)
	ids := newFullFlowIDs(t)
	now := int64(1_700_000_000_000)
	clientA := newFullFlowClient("client-a", newSyncTestClient(), accessToken, ids.colID)
	clientB := newFullFlowClient("client-b", newSyncTestClient(), accessToken, ids.colID)

	upserts := fullFlowUpsertChanges(ids, now)
	assignedUpserts := clientA.pushAllLocalChanges(t, upserts)

	// 这里因为每个实体各传一个 change，所以总共 7 个 change，最后的 syncCursorUSN 应该是 8
	require.Equal(t, int64(8), clientA.syncCursorUSN)
	requireSessionReleased(t, userID)

	pullHsk, firstPullChanges := clientB.pullAllRemoteChanges(t)
	require.Equal(t, int64(8), clientB.syncCursorUSN)
	requirePulledChangesMatchClientState(t, assignedUpserts, firstPullChanges)

	deletedAt := now + 10_000
	deletes := []*syncv1.SyncChange{
		deleteChange(ids.pcsNoteID, syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE, deletedAt),
		deleteChange(ids.noteID, syncv1.EntityType_ENTITY_TYPE_NOTE, deletedAt+1),
		deleteChange(ids.cardID, syncv1.EntityType_ENTITY_TYPE_CARD, deletedAt+2),
	}
	assignedDeletes := clientB.pushChangesInSession(t, pullHsk.GetSessionId(), 1, deletes)
	clientB.finish(t, pullHsk.GetSessionId())
	require.Equal(t, int64(11), clientB.syncCursorUSN)
	requireSessionReleased(t, userID)

	deletePullHsk, deletePullChanges := clientA.pullAllRemoteChanges(t)
	require.Equal(t, int64(11), clientA.syncCursorUSN)
	requirePulledChangesMatchClientState(t, assignedDeletes, deletePullChanges)
	clientA.finish(t, deletePullHsk.GetSessionId())
	requireSessionReleased(t, userID)
}

// TestHandshakeLockedByOtherClient 测试当一个客户端已经在拉取时，另一个客户端尝试 handshake 会返回 LOCKED_BY_OTHER_CLIENT
func TestHandshakeLockedByOtherClient(t *testing.T) {
	resetEnv(t)
	userID := createSyncTestUser(t, "flowlock")
	clientA := newSyncTestClient()
	clientB := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	collectionID := pullTestUUID(t)
	deviceID := uuid.Must(uuid.NewV7())

	firstHsk := startPushSync(t, clientA, accessToken, collectionID)
	lockedResp := sendHandshake(t, clientB, accessToken, &syncv1.HandshakeRequest{
		DeviceId:            deviceID[:],
		DeviceName:          "integration-test-device-2",
		CollectionId:        collectionID,
		ClientSyncCursorUsn: 1,
		ProtocolVersion:     1,
		DbSchemaVersion:     1,
		ClientNow:           time.Now().UnixMilli(),
		HasLocalChanges:     true,
	})

	require.NotEmpty(t, firstHsk.GetSessionId())
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_LOCKED_BY_OTHER_CLIENT, lockedResp.Status)
	require.Nil(t, lockedResp.SessionId)
}

type fullFlowClient struct {
	name          string
	rpc           syncv1connect.SyncServiceClient
	accessToken   string
	collectionID  []byte
	syncCursorUSN int64
}

func newFullFlowClient(name string, rpc syncv1connect.SyncServiceClient, accessToken string, collectionID []byte) *fullFlowClient {
	return &fullFlowClient{
		name:          name,
		rpc:           rpc,
		accessToken:   accessToken,
		collectionID:  collectionID,
		syncCursorUSN: 1,
	}
}

func (c *fullFlowClient) handshake(t *testing.T, hasLocalChanges bool) *syncv1.HandshakeResponse {
	t.Helper()
	deviceID := uuid.Must(uuid.NewV7())
	return sendHandshake(t, c.rpc, c.accessToken, &syncv1.HandshakeRequest{
		DeviceId:            deviceID[:],
		DeviceName:          c.name,
		CollectionId:        c.collectionID,
		ClientSyncCursorUsn: c.syncCursorUSN,
		ProtocolVersion:     1,
		DbSchemaVersion:     1,
		ClientNow:           time.Now().UnixMilli(),
		HasLocalChanges:     hasLocalChanges,
	})
}

func (c *fullFlowClient) pushAllLocalChanges(t *testing.T, changes []*syncv1.SyncChange) []*syncv1.SyncChange {
	t.Helper()
	hskResp := c.handshake(t, true)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES, hskResp.Status)
	require.Equal(t, c.syncCursorUSN, hskResp.ServerSyncCursorUsn)
	require.NotEmpty(t, hskResp.GetSessionId())

	assignedChanges := c.pushChangesInSession(t, hskResp.GetSessionId(), 1, changes)
	c.finish(t, hskResp.GetSessionId())
	return assignedChanges
}

func (c *fullFlowClient) pushChangesInSession(t *testing.T, sessionID string, batchSeq int32, changes []*syncv1.SyncChange) []*syncv1.SyncChange {
	t.Helper()
	pushResp := sendPush(t, c.rpc, c.accessToken, &syncv1.PushRequest{
		SessionId: sessionID,
		BatchSeq:  batchSeq,
		Changes:   changes,
		LastBatch: true,
	})
	assignedChanges := requirePushAssignedUSNs(t, changes, pushResp.Changes, c.syncCursorUSN)
	c.syncCursorUSN += int64(len(changes))
	return assignedChanges
}

func (c *fullFlowClient) pullAllRemoteChanges(t *testing.T) (*syncv1.HandshakeResponse, []*syncv1.SyncChange) {
	t.Helper()
	hskResp := c.handshake(t, false)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_NEED_PULL, hskResp.Status)
	require.NotEmpty(t, hskResp.GetSessionId())
	require.Greater(t, hskResp.ServerSyncCursorUsn, c.syncCursorUSN)

	var changes []*syncv1.SyncChange
	for batchSeq := int32(1); ; batchSeq++ {
		pullResp := sendPull(t, c.rpc, c.accessToken, hskResp.GetSessionId(), batchSeq)
		require.Equal(t, batchSeq, pullResp.BatchSeq)
		changes = append(changes, pullResp.Changes...)
		if pullResp.LastBatch {
			break
		}
	}
	c.syncCursorUSN = hskResp.ServerSyncCursorUsn
	return hskResp, changes
}

func (c *fullFlowClient) finish(t *testing.T, sessionID string) {
	t.Helper()
	sendFinishSync(t, c.rpc, c.accessToken, sessionID)
}

type fullFlowIDs struct {
	colID       []byte
	deckID      []byte
	noteTypeID  []byte
	noteID      []byte
	pcsNoteID   []byte
	cardID      []byte
	reviewLogID []byte
}

func newFullFlowIDs(t *testing.T) fullFlowIDs {
	t.Helper()
	return fullFlowIDs{
		colID:       pullTestUUID(t),
		deckID:      pullTestUUID(t),
		noteTypeID:  pullTestUUID(t),
		noteID:      pullTestUUID(t),
		pcsNoteID:   pullTestUUID(t),
		cardID:      pullTestUUID(t),
		reviewLogID: pullTestUUID(t),
	}
}

// fullFlowUpsertChanges 会给每一个实体类型生成一个 upsert change，usn 都是 -1，表示客户端本地的 change
// 一共有 7 个 upsert change
func fullFlowUpsertChanges(ids fullFlowIDs, now int64) []*syncv1.SyncChange {
	senseID := int32(1)
	lastReview := now + 6
	return []*syncv1.SyncChange{
		{
			EntityId:   ids.colID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_Collection{Collection: &syncv1.CollectionPayload{
				SqliteSchemaVersion: 15,
				CreatedAt:           now,
				UpdatedAt:           now + 1,
				ConfigJson:          `{"flow":"collection"}`,
			}},
		},
		{
			EntityId:   ids.deckID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_Deck{Deck: &syncv1.DeckPayload{
				Name:            "deck",
				UpdatedAt:       now + 2,
				NewCardsPerDay:  20,
				NewLearnedToday: 1,
				LearnedToday:    2,
				ReviewedToday:   3,
				ConfigJson:      `{"flow":"deck"}`,
			}},
		},
		{
			EntityId:   ids.noteTypeID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_NoteType{NoteType: &syncv1.NoteTypePayload{
				Name:             "basic",
				PresetTemplateId: 1,
				UpdatedAt:        now + 3,
				NoteTemplateJson: `{"flow":"note_type"}`,
			}},
		},
		{
			EntityId:   ids.noteID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_NOTE,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_Note{Note: &syncv1.NotePayload{
				NoteTypeId: ids.noteTypeID,
				CreatedAt:  now,
				UpdatedAt:  now + 4,
				SenseId:    &senseID,
				FieldsJson: `{"front":"note"}`,
			}},
		},
		{
			EntityId:   ids.pcsNoteID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_ProcessingNote{ProcessingNote: &syncv1.ProcessingNotePayload{
				NoteTypeId: ids.noteTypeID,
				CreatedAt:  now,
				UpdatedAt:  now + 5,
				SenseId:    &senseID,
				FieldsJson: `{"front":"processing"}`,
			}},
		},
		{
			EntityId:   ids.cardID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_CARD,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_Card{Card: &syncv1.CardPayload{
				NoteId:        ids.noteID,
				DeckId:        ids.deckID,
				UpdatedAt:     now + 6,
				Difficulty:    2.5,
				Stability:     3.5,
				ScheduledDays: 1,
				Due:           now + 20_000,
				LastReview:    &lastReview,
				Lapses:        0,
				LearningSteps: 0,
				Repetitions:   1,
				State:         1,
				Queue:         1,
			}},
		},
		{
			EntityId:   ids.reviewLogID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_ReviewLog{ReviewLog: &syncv1.ReviewLogPayload{
				CardId:        ids.cardID,
				ReviewTime:    now + 7,
				ScheduledDays: 1,
				Rating:        3,
				Difficulty:    2.6,
				Stability:     3.6,
				LearningSteps: 0,
				State:         1,
				Duration:      30,
			}},
		},
	}
}

func deleteChange(entityID []byte, entityType syncv1.EntityType, deletedAt int64) *syncv1.SyncChange {
	return &syncv1.SyncChange{
		EntityId:   entityID,
		EntityType: entityType,
		Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
		Usn:        -1,
		DeletedAt:  &deletedAt,
	}
}

func sendPush(t *testing.T, client syncv1connect.SyncServiceClient, accessToken string, req *syncv1.PushRequest) *syncv1.PushResponse {
	t.Helper()
	resp, err := client.Push(t.Context(), newAuthorizedRequest(req, accessToken))
	require.NoError(t, err)
	return resp.Msg
}

func sendFinishSync(t *testing.T, client syncv1connect.SyncServiceClient, accessToken string, sessionID string) *syncv1.FinishSyncResponse {
	t.Helper()
	resp, err := client.FinishSync(t.Context(), newAuthorizedRequest(&syncv1.FinishSyncRequest{
		SessionId: sessionID,
	}, accessToken))
	require.NoError(t, err)
	require.Positive(t, resp.Msg.ServerFinishedAt)
	return resp.Msg
}

func requirePushAssignedUSNs(t *testing.T, pushedChanges, assignedResponses []*syncv1.SyncChange, wantStartUSN int64) []*syncv1.SyncChange {
	t.Helper()
	require.Len(t, assignedResponses, len(pushedChanges))
	assignedChanges := make([]*syncv1.SyncChange, 0, len(pushedChanges))
	for i, pushedChange := range pushedChanges {
		assignedResponse := assignedResponses[i]
		require.Equal(t, pushedChange.GetEntityId(), assignedResponse.GetEntityId())
		require.Equal(t, pushedChange.GetEntityType(), assignedResponse.GetEntityType())
		require.Equal(t, syncv1.ChangeOp_CHANGE_OP_ASSIGN_USN, assignedResponse.GetOp())
		require.Equal(t, wantStartUSN+int64(i), assignedResponse.GetUsn())

		assignedChange := cloneClientChange(pushedChange)
		assignedChange.Usn = assignedResponse.GetUsn()
		assignedChanges = append(assignedChanges, assignedChange)
	}
	return assignedChanges
}

func cloneClientChange(change *syncv1.SyncChange) *syncv1.SyncChange {
	return proto.Clone(change).(*syncv1.SyncChange)
}

func requirePulledChangesMatchClientState(t *testing.T, wantChanges, gotChanges []*syncv1.SyncChange) {
	t.Helper()
	require.Len(t, gotChanges, len(wantChanges))
	for _, wantChange := range wantChanges {
		gotChange := requirePulledChange(t, gotChanges, wantChange.GetEntityId(), wantChange.GetEntityType())
		requireClientVisibleChangeEqual(t, wantChange, gotChange)
	}
}

func requireClientVisibleChangeEqual(t *testing.T, wantChange, gotChange *syncv1.SyncChange) {
	t.Helper()
	require.Equal(t, wantChange.GetEntityId(), gotChange.GetEntityId())
	require.Equal(t, wantChange.GetEntityType(), gotChange.GetEntityType())
	require.Equal(t, wantChange.GetOp(), gotChange.GetOp())
	require.Equal(t, wantChange.GetUsn(), gotChange.GetUsn())

	switch wantChange.GetOp() {
	case syncv1.ChangeOp_CHANGE_OP_UPSERT:
		require.NotNil(t, gotChange.GetPayload())
		requireUpsertPayloadEqual(t, wantChange, gotChange)
	case syncv1.ChangeOp_CHANGE_OP_DELETE:
		require.Nil(t, gotChange.GetPayload())
		require.NotNil(t, gotChange.DeletedAt)
		require.Equal(t, wantChange.GetDeletedAt(), gotChange.GetDeletedAt())
	default:
		require.Failf(t, "unexpected client visible op", "op=%s", wantChange.GetOp())
	}
}

func requireUpsertPayloadEqual(t *testing.T, wantChange, gotChange *syncv1.SyncChange) {
	t.Helper()
	switch wantChange.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		requireCollectionPayloadEqual(t, wantChange.GetCollection(), gotChange.GetCollection())
	case syncv1.EntityType_ENTITY_TYPE_DECK:
		requireDeckPayloadEqual(t, wantChange.GetDeck(), gotChange.GetDeck())
	case syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE:
		requireNoteTypePayloadEqual(t, wantChange.GetNoteType(), gotChange.GetNoteType())
	case syncv1.EntityType_ENTITY_TYPE_NOTE:
		requireNotePayloadEqual(t, wantChange.GetNote(), gotChange.GetNote())
	case syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE:
		requireProcessingNotePayloadEqual(t, wantChange.GetProcessingNote(), gotChange.GetProcessingNote())
	case syncv1.EntityType_ENTITY_TYPE_CARD:
		requireCardPayloadEqual(t, wantChange.GetCard(), gotChange.GetCard())
	case syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG:
		requireReviewLogPayloadEqual(t, wantChange.GetReviewLog(), gotChange.GetReviewLog())
	default:
		require.Failf(t, "unexpected upsert entity type", "entityType=%s", wantChange.GetEntityType())
	}
}

func requireCollectionPayloadEqual(t *testing.T, want, got *syncv1.CollectionPayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetSqliteSchemaVersion(), got.GetSqliteSchemaVersion())
	require.Equal(t, want.GetCreatedAt(), got.GetCreatedAt())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	require.JSONEq(t, want.GetConfigJson(), got.GetConfigJson())
}

func requireDeckPayloadEqual(t *testing.T, want, got *syncv1.DeckPayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetName(), got.GetName())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	require.Equal(t, want.GetNewCardsPerDay(), got.GetNewCardsPerDay())
	require.Equal(t, want.GetNewLearnedToday(), got.GetNewLearnedToday())
	require.Equal(t, want.GetLearnedToday(), got.GetLearnedToday())
	require.Equal(t, want.GetReviewedToday(), got.GetReviewedToday())
	require.JSONEq(t, want.GetConfigJson(), got.GetConfigJson())
}

func requireNoteTypePayloadEqual(t *testing.T, want, got *syncv1.NoteTypePayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetName(), got.GetName())
	require.Equal(t, want.GetPresetTemplateId(), got.GetPresetTemplateId())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	require.JSONEq(t, want.GetNoteTemplateJson(), got.GetNoteTemplateJson())
}

func requireNotePayloadEqual(t *testing.T, want, got *syncv1.NotePayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetNoteTypeId(), got.GetNoteTypeId())
	require.Equal(t, want.GetCreatedAt(), got.GetCreatedAt())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	requireOptionalInt32Equal(t, want.SenseId, got.SenseId)
	require.JSONEq(t, want.GetFieldsJson(), got.GetFieldsJson())
}

func requireProcessingNotePayloadEqual(t *testing.T, want, got *syncv1.ProcessingNotePayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetNoteTypeId(), got.GetNoteTypeId())
	require.Equal(t, want.GetCreatedAt(), got.GetCreatedAt())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	requireOptionalInt32Equal(t, want.SenseId, got.SenseId)
	require.JSONEq(t, want.GetFieldsJson(), got.GetFieldsJson())
}

func requireCardPayloadEqual(t *testing.T, want, got *syncv1.CardPayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetNoteId(), got.GetNoteId())
	require.Equal(t, want.GetDeckId(), got.GetDeckId())
	require.Equal(t, want.GetUpdatedAt(), got.GetUpdatedAt())
	require.Equal(t, want.GetDifficulty(), got.GetDifficulty())
	require.Equal(t, want.GetStability(), got.GetStability())
	require.Equal(t, want.GetScheduledDays(), got.GetScheduledDays())
	require.Equal(t, want.GetDue(), got.GetDue())
	requireOptionalInt64Equal(t, want.LastReview, got.LastReview)
	require.Equal(t, want.GetLapses(), got.GetLapses())
	require.Equal(t, want.GetLearningSteps(), got.GetLearningSteps())
	require.Equal(t, want.GetRepetitions(), got.GetRepetitions())
	require.Equal(t, want.GetState(), got.GetState())
	require.Equal(t, want.GetQueue(), got.GetQueue())
}

func requireReviewLogPayloadEqual(t *testing.T, want, got *syncv1.ReviewLogPayload) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.GetCardId(), got.GetCardId())
	require.Equal(t, want.GetReviewTime(), got.GetReviewTime())
	require.Equal(t, want.GetScheduledDays(), got.GetScheduledDays())
	require.Equal(t, want.GetRating(), got.GetRating())
	require.Equal(t, want.GetDifficulty(), got.GetDifficulty())
	require.Equal(t, want.GetStability(), got.GetStability())
	require.Equal(t, want.GetLearningSteps(), got.GetLearningSteps())
	require.Equal(t, want.GetState(), got.GetState())
	require.Equal(t, want.GetDuration(), got.GetDuration())
}

func requireOptionalInt32Equal(t *testing.T, want, got *int32) {
	t.Helper()
	require.Equal(t, want != nil, got != nil)
	if want != nil {
		require.Equal(t, *want, *got)
	}
}

func requireOptionalInt64Equal(t *testing.T, want, got *int64) {
	t.Helper()
	require.Equal(t, want != nil, got != nil)
	if want != nil {
		require.Equal(t, *want, *got)
	}
}

func requirePulledChangesContainUpsert(t *testing.T, changes []*syncv1.SyncChange, entityID []byte, entityType syncv1.EntityType) {
	t.Helper()
	change := requirePulledChange(t, changes, entityID, entityType)
	require.Equal(t, syncv1.ChangeOp_CHANGE_OP_UPSERT, change.Op)
	require.NotNil(t, change.GetPayload())
}

func requirePulledChange(t *testing.T, changes []*syncv1.SyncChange, entityID []byte, entityType syncv1.EntityType) *syncv1.SyncChange {
	t.Helper()
	for _, change := range changes {
		if change.GetEntityType() == entityType && string(change.GetEntityId()) == string(entityID) {
			return change
		}
	}
	require.Failf(t, "missing pulled change", "entityType=%s entityID=%x", entityType, entityID)
	return nil
}

func requireSessionReleased(t *testing.T, userID int64) {
	t.Helper()
	exists, err := suite.Env.RDB.Exists(t.Context(), ss.RdbSessionKey(userID)).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}
