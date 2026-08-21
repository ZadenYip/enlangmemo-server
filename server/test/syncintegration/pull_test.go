package syncintegration

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/sync/collector"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
	"github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1/syncv1connect"
)

// TestPullCollectionCardReviewLogSuccess 测试一次拉取 collection、cards、review_logs 后，
// Pull 返回 last_batch=true，并且 session 进入 AWAITING_PUSH_OR_FINISH。
func TestPullCollectionCardReviewLogSuccess(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-basic")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)

	time := time.Now().UnixMilli()

	inserter := newPullTestInserter(t, userID, 1)
	collectionChange := inserter.InsertCollection(4)
	noteChange := inserter.InsertNoteWithUSN(0, time, `{"front":"basic"}`)
	cardChange := inserter.InsertCard(noteChange.entityID, time)
	inserter.InsertReviewLog(cardChange.entityID, time)
	handshakeResp := startPullSync(t, client, accessToken, collectionChange.entityID, 1, 4)

	pullResp := sendPull(t, client, accessToken, handshakeResp.GetSessionId(), 1)

	require.True(t, pullResp.LastBatch)
	require.Equal(t, int32(1), pullResp.BatchSeq)
	require.Equal(t, int64(3), pullResp.BatchMaxUsn)
	require.Equal(t, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		syncv1.EntityType_ENTITY_TYPE_CARD,
		syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG,
	}, pullTestExactChangeTypes(pullResp.Changes))
	requirePullSession(t, userID, ss.SyncSessionStateAwaitingPushOrFinish, 1, nil, 4)
}

// TestPullNoteSizeExceededKeepsNoteInQueue 测试拉取 notes 和 cards note 时数据量超过 batchMaxSize 且 note 实体没拉取完时，
// entityQueue 首个实体是 note，不会提前移除 note，队首变为 card
func TestPullNoteSizeExceededKeepsNoteInQueue(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-a")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)

	time := time.Now().UnixMilli()

	inserter := newPullTestInserter(t, userID, 2)
	colChange := inserter.InsertCollectionWithUSN(0, 5)
	ntChange1 := inserter.InsertNote(time, genJSONString(collector.MaxBatchSize))
	inserter.InsertNote(time, `{"tail":true}`)
	inserter.InsertCard(ntChange1.entityID, time)
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 1, 5)

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.False(t, pullResp.LastBatch)
	requirePullChangesAllEntityType(t, pullResp.Changes, syncv1.EntityType_ENTITY_TYPE_NOTE)

	requirePullSession(t, userID, ss.SyncSessionStatePulling, 2, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_NOTE,
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, 3)
}

// TestPullNoteFinishedAfterSizeExceededRemovesNoteKeepsCard 测试拉取 notes 和 cards 时，
// 刚好拉取完了 notes 但达到 batch 大小上限，
// entityQueue 应该移除 note 实体类型，队首变为 card
func TestPullNoteFinishedAfterSizeExceededRemovesNoteKeepsCard(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-b")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	time := time.Now().UnixMilli()

	inserter := newPullTestInserter(t, userID, 2)
	colChange := inserter.InsertCollectionWithUSN(0, 4)
	ntChange := inserter.InsertNote(time, genJSONString(collector.MaxBatchSize))
	inserter.InsertCard(ntChange.entityID, time)
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 1, 4)

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.False(t, pullResp.LastBatch)
	requirePullChangesAllEntityType(t, pullResp.Changes, syncv1.EntityType_ENTITY_TYPE_NOTE)

	// note 实体类型已经拉完，队首变为 card
	requirePullSession(t, userID, ss.SyncSessionStatePulling, 2, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, 1)
}

// TestPullNoteLimitReachedAndSizeExceededRemovesNoteKeepsCard 测试拉取 notes 和 cards，
// note 数量达到一次性拉取上限且 batch 大小也达到上限时，
// note 应该能从 entityQueue 移除但 card 仍然存在队列保留到下一批
func TestPullNoteLimitReachedAndSizeExceededRemovesNoteKeepsCard(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-c")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	fieldSize := pullTestNoteFieldsSizeThatFillsBatchOnLimit()

	time := time.Now().UnixMilli()

	inserter := newPullTestInserter(t, userID, 2)
	collectionChange := inserter.InsertCollectionWithUSN(0, int64(collector.LimitNote+3))
	var firstNoteID []byte
	for i := 0; i < collector.LimitNote; i++ {
		noteChange := inserter.InsertNote(time, genJSONString(fieldSize))
		if i == 0 {
			firstNoteID = noteChange.entityID
		}
	}
	inserter.InsertCard(firstNoteID, time+1)
	hskResp := startPullSync(t, client, accessToken, collectionChange.entityID, 1, int64(collector.LimitNote+3))

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.False(t, pullResp.LastBatch)
	require.Len(t, pullResp.Changes, collector.LimitNote)
	requirePullChangesAllEntityType(t, pullResp.Changes, syncv1.EntityType_ENTITY_TYPE_NOTE)
	requirePullSession(t, userID, ss.SyncSessionStatePulling, 2, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, 1)
}

// TestPullNoteLimitReachedAndSizeExceededKeepsNoteWhenHasMore 测试拉取 notes 和 cards，
// note 数量达到一次性拉取上限且 batch 大小也达到上限，但 note 还有剩余数据时，
// note 不应该从 entityQueue 移除，下一批继续从 note 开始拉取。
func TestPullNoteLimitReachedAndSizeExceededKeepsNoteWhenHasMore(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-e")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	fieldSize := pullTestNoteFieldsSizeThatFillsBatchOnLimit()
	time := time.Now().UnixMilli()

	// 创建比拉取上限多 1 条 note，确保拉取时会触及 limit
	noteCount := collector.LimitNote + 1

	// 1 是 collection，2 ~ noteCount + 2 是 note
	colSync := int64(noteCount + 3)

	inserter := newPullTestInserter(t, userID, 1)
	colChange := inserter.InsertCollection(colSync)
	var firstNoteID []byte
	for i := 0; i < noteCount; i++ {
		noteChange := inserter.InsertNote(time, genJSONString(fieldSize))
		if i == 0 {
			firstNoteID = noteChange.entityID
		}
	}
	inserter.InsertCard(firstNoteID, time+1)
	// 从 2 拉起，1 是集合
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 2, colSync)

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.False(t, pullResp.LastBatch)

	// 测试是不是拉取达到上限
	require.Len(t, pullResp.Changes, collector.LimitNote)
	syncCursorUSN := pullResp.BatchMaxUsn + 1

	requirePullChangesAllEntityType(t, pullResp.Changes, syncv1.EntityType_ENTITY_TYPE_NOTE)
	requirePullSession(t, userID, ss.SyncSessionStatePulling, 2, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_NOTE,
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, syncCursorUSN)
}

// TestPullNoteLimitReachedWithoutSizeExceededContinuesSameEntity 测试拉取 notes 时，
// note 数量超过单次查询上限但 batch 大小未满，Pull 会继续从新的 cursor 拉同一实体类型，
// 然后再拉取后续 card
func TestPullNoteLimitReachedWithoutSizeExceededContinuesSameEntity(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-limit")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	time := time.Now().UnixMilli()

	noteCount := collector.LimitNote + 1
	colSync := int64(noteCount + 3)

	inserter := newPullTestInserter(t, userID, 2)
	colChange := inserter.InsertCollectionWithUSN(0, colSync)
	var firstNoteID []byte
	for i := 0; i < noteCount; i++ {
		noteChange := inserter.InsertNote(time, `{"front":"small"}`)
		if i == 0 {
			firstNoteID = noteChange.entityID
		}
	}
	cardChange := inserter.InsertCard(firstNoteID, time+1)
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 1, colSync)

	pullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	require.True(t, pullResp.LastBatch)
	require.Len(t, pullResp.Changes, noteCount+1)
	require.Equal(t, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_NOTE,
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, pullTestChangeTypeRuns(pullResp.Changes))
	require.Equal(t, cardChange.entityID, pullResp.Changes[len(pullResp.Changes)-1].EntityId)
	require.Equal(t, colSync-1, pullResp.BatchMaxUsn)
	requirePullSession(t, userID, ss.SyncSessionStateAwaitingPushOrFinish, 1, nil, colSync)
}

// TestPullManyNotesSecondBatchFinishesThenPullsCard 测试 note 数量很多导致第一批触及 limit，
// 第二批拉完剩余 note 后会继续正常拉取 card，并最终完成所有 pull。
func TestPullManyNotesSecondBatchFinishesThenPullsCard(t *testing.T) {
	resetEnv(t)
	resetPullTestEntityTables(t)
	userID := createSyncTestUser(t, "pull-note-d")
	client := newSyncTestClient()
	accessToken := newSyncTestAccessToken(t, userID)
	noteCount := collector.LimitNote + 6
	fieldSize := pullTestNoteFieldsSizeThatFillsBatchOnLimit()
	time := time.Now().UnixMilli()

	colSync := int64(noteCount + 3)

	inserter := newPullTestInserter(t, userID, 2)
	colChange := inserter.InsertCollectionWithUSN(0, colSync)
	var firstNoteID []byte
	for i := 0; i < noteCount; i++ {
		noteChange := inserter.InsertNote(time, genJSONString(fieldSize))
		if i == 0 {
			firstNoteID = noteChange.entityID
		}
	}
	cardChange := inserter.InsertCard(firstNoteID, time)
	hskResp := startPullSync(t, client, accessToken, colChange.entityID, 1, colSync)

	firstPullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 1)

	// 第一批拉取 note，触及 limit，返回 last_batch=false
	require.False(t, firstPullResp.LastBatch)
	require.Len(t, firstPullResp.Changes, collector.LimitNote)
	requirePullChangesAllEntityType(t, firstPullResp.Changes, syncv1.EntityType_ENTITY_TYPE_NOTE)
	requirePullSession(t, userID, ss.SyncSessionStatePulling, 2, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_NOTE,
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, int64(collector.LimitNote+2))

	// 第二批拉取剩余 note 和 card，返回 last_batch=true
	secondPullResp := sendPull(t, client, accessToken, hskResp.GetSessionId(), 2)
	require.True(t, secondPullResp.LastBatch)
	require.Len(t, secondPullResp.Changes, 7)
	require.Equal(t, []syncv1.EntityType{
		syncv1.EntityType_ENTITY_TYPE_NOTE,
		syncv1.EntityType_ENTITY_TYPE_CARD,
	}, pullTestChangeTypeRuns(secondPullResp.Changes))
	require.Equal(t, cardChange.entityID, secondPullResp.Changes[len(secondPullResp.Changes)-1].EntityId)
	requirePullSession(t, userID, ss.SyncSessionStateAwaitingPushOrFinish, 1, nil, colSync)
}

func startPullSync(t *testing.T, client syncv1connect.SyncServiceClient, accessToken string, collectionID []byte, clientCursorUSN, serverCursorUSN int64) *syncv1.HandshakeResponse {
	t.Helper()
	deviceID := uuid.Must(uuid.NewV7())
	resp := sendHandshake(t, client, accessToken, &syncv1.HandshakeRequest{
		DeviceId:            deviceID[:],
		DeviceName:          "integration-test-device",
		CollectionId:        collectionID,
		ClientSyncCursorUsn: clientCursorUSN,
		ProtocolVersion:     1,
		DbSchemaVersion:     1,
		ClientNow:           time.Now().UnixMilli(),
		HasLocalChanges:     false,
	})
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_NEED_PULL, resp.Status)
	require.NotEmpty(t, resp.GetSessionId())
	require.Equal(t, serverCursorUSN, resp.ServerSyncCursorUsn)
	return resp
}

func sendPull(t *testing.T, client syncv1connect.SyncServiceClient, accessToken string, sessionID string, batchSeq int32) *syncv1.PullResponse {
	t.Helper()
	resp, err := client.Pull(t.Context(), newAuthorizedRequest(&syncv1.PullRequest{
		SessionId: sessionID,
		BatchSeq:  batchSeq,
	}, accessToken))
	require.NoError(t, err)
	return resp.Msg
}

func requirePullSession(t *testing.T, userID int64, wantState ss.SessionState, wantBatchSeq int64, wantQueue []syncv1.EntityType, wantSyncCursorUSN int64) {

	t.Helper()
	key := ss.RdbSessionKey(userID)
	gotState, err := suite.Env.RDB.HGet(t.Context(), key, "state").Int64()
	require.NoError(t, err)
	require.Equal(t, int64(wantState), gotState)
	gotBatchSeq, err := suite.Env.RDB.HGet(t.Context(), key, "expected_batch_seq").Int64()
	require.NoError(t, err)
	require.Equal(t, wantBatchSeq, gotBatchSeq)
	gotSyncCursorUSN, err := suite.Env.RDB.HGet(t.Context(), key, "sync_cursor_usn").Int64()
	require.NoError(t, err)
	require.Equal(t, wantSyncCursorUSN, gotSyncCursorUSN)
	gotQueue, err := suite.Env.RDB.HGet(t.Context(), key, "pull_entity_queue").Result()
	if len(wantQueue) == 0 {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	queue := pullTestTypeQueueToString(wantQueue)
	require.Equal(t, queue, gotQueue)
}

func pullTestTypeQueueToString(typeQueue []syncv1.EntityType) string {
	if len(typeQueue) == 0 {
		return ""
	}
	strs := make([]string, len(typeQueue))
	for i, entityType := range typeQueue {
		strs[i] = strconv.Itoa(int(entityType))
	}
	return strings.Join(strs, ",")
}

func pullTestNoteFieldsSizeThatFillsBatchOnLimit() int {
	noteFixedSize := collector.NoteIDSize + collector.NoteNoteTypeIDSize + collector.NoteUsnSize +
		collector.NoteCreatedAtSize + collector.NoteUpdatedAtSize + collector.NoteSenseIDSize +
		collector.NoteIsDeletedSize

	// 构造一个边界：前 LimitNote-1 条 note 加起来还没满 batch，
	// 第 LimitNote 条 note 加入后刚好让 collector.IsFull() 为 true。
	maxSingleNoteSizeBeforeLastNote := (collector.MaxBatchSize - 1) / (collector.LimitNote - 1)
	return maxSingleNoteSizeBeforeLastNote - noteFixedSize
}

func pullTestExactChangeTypes(changes []*syncv1.SyncChange) []syncv1.EntityType {
	types := make([]syncv1.EntityType, 0, len(changes))
	for _, change := range changes {
		types = append(types, change.EntityType)
	}
	return types
}

func requirePullChangesAllEntityType(t *testing.T, changes []*syncv1.SyncChange, wantEntityType syncv1.EntityType) {
	t.Helper()
	for _, change := range changes {
		require.Equal(t, wantEntityType, change.EntityType)
	}
}

func pullTestChangeTypeRuns(changes []*syncv1.SyncChange) []syncv1.EntityType {
	types := make([]syncv1.EntityType, 0, len(changes))
	for _, entityType := range pullTestExactChangeTypes(changes) {
		if len(types) == 0 || types[len(types)-1] != entityType {
			types = append(types, entityType)
		}
	}
	return types
}
