package syncintegration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type pullTestEntityChange struct {
	entityID   []byte
	entityType syncv1.EntityType
	usn        int64
	updatedAt  int64
}

type pullTestInserter struct {
	t       *testing.T
	userID  int64
	nextUSN int64
}

func newPullTestInserter(t *testing.T, userID int64, startUSN int64) *pullTestInserter {
	t.Helper()
	return &pullTestInserter{
		t:       t,
		userID:  userID,
		nextUSN: startUSN,
	}
}

func (i *pullTestInserter) NextUSN() int64 {
	return i.nextUSN
}

func (i *pullTestInserter) insertSyncUnit(change pullTestEntityChange) {
	i.t.Helper()
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO sync_units (user_id, entity_id, entity_type, op, usn, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		i.userID,
		change.entityID,
		int32(change.entityType),
		int32(syncv1.ChangeOp_CHANGE_OP_UPSERT),
		change.usn,
		change.updatedAt,
	)
	require.NoError(i.t, err)
}

func (i *pullTestInserter) recordChange(change pullTestEntityChange) pullTestEntityChange {
	i.t.Helper()
	i.insertSyncUnit(change)
	i.nextUSN++
	return change
}

type syncTestCollection struct {
	ID                  []byte
	USN                 int64
	SQLiteSchemaVersion int32
	LastSyncTime        int64
	SyncCursorUSN       int64
	CreatedAt           int64
	UpdatedAt           int64
	ConfigJSON          string
	IsDeleted           bool
}

func getSyncTestCollection(t *testing.T, userID int64, collectionID []byte) syncTestCollection {
	t.Helper()
	var got syncTestCollection
	err := suite.Env.DB.QueryRowContext(
		t.Context(),
		`SELECT id, usn, sqlite_schema_version, last_sync_time, sync_cursor_usn,
				created_at, updated_at, config, is_deleted
			 FROM collections
		 WHERE user_id = ? AND id = ?`,
		userID,
		collectionID,
	).Scan(
		&got.ID,
		&got.USN,
		&got.SQLiteSchemaVersion,
		&got.LastSyncTime,
		&got.SyncCursorUSN,
		&got.CreatedAt,
		&got.UpdatedAt,
		&got.ConfigJSON,
		&got.IsDeleted,
	)
	require.NoError(t, err)
	return got
}

func resetPullTestEntityTables(t *testing.T) {
	t.Helper()
	_, err := suite.Env.DB.ExecContext(t.Context(), `
		TRUNCATE TABLE review_logs;
		TRUNCATE TABLE cards;
		TRUNCATE TABLE processing_notes;
		TRUNCATE TABLE notes;
		TRUNCATE TABLE note_types;
		TRUNCATE TABLE decks;
	`)
	require.NoError(t, err)
}

func (i *pullTestInserter) InsertDeck(updatedAt int64) pullTestEntityChange {
	i.t.Helper()
	deckID := pullTestUUID(i.t)
	usn := i.nextUSN
	newCardsPerDay := 20
	newLearnedToday := 1
	learnedToday := 2
	reviewedToday := 3
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO decks (
			user_id, id, usn, name, updated_at,
			new_cards_per_day, new_learned_today, learned_today, reviewed_today,
			config, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		i.userID,
		deckID,
		usn,
		"test deck",
		updatedAt,
		newCardsPerDay,
		newLearnedToday,
		learnedToday,
		reviewedToday,
		`{"deck":true}`,
	)
	require.NoError(i.t, err)
	return i.recordChange(pullTestEntityChange{
		entityID:   deckID,
		entityType: syncv1.EntityType_ENTITY_TYPE_DECK,
		usn:        usn,
		updatedAt:  updatedAt,
	})
}

func (i *pullTestInserter) InsertNoteType(updatedAt int64, noteTemplateJSON string) pullTestEntityChange {
	i.t.Helper()
	noteTypeID := pullTestUUID(i.t)
	usn := i.nextUSN
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO note_types (
			user_id, id, usn, name, preset_template_id, updated_at, note_template, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0)`,
		i.userID,
		noteTypeID,
		usn,
		"basic",
		1,
		updatedAt,
		noteTemplateJSON,
	)
	require.NoError(i.t, err)
	return i.recordChange(pullTestEntityChange{
		entityID:   noteTypeID,
		entityType: syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE,
		usn:        usn,
		updatedAt:  updatedAt,
	})
}

func (i *pullTestInserter) InsertProcessingNote(updatedAt int64, fieldsJSON string) pullTestEntityChange {
	i.t.Helper()
	pcsNoteID := pullTestUUID(i.t)
	noteTypeID := pullTestUUID(i.t)
	usn := i.nextUSN
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO processing_notes (
			user_id, id, note_type_id, usn, created_at, updated_at, sense_id, fields, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		i.userID,
		pcsNoteID,
		noteTypeID,
		usn,
		1_700_000_000_000,
		updatedAt,
		int32(1),
		fieldsJSON,
	)
	require.NoError(i.t, err)
	return i.recordChange(pullTestEntityChange{
		entityID:   pcsNoteID,
		entityType: syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE,
		usn:        usn,
		updatedAt:  updatedAt,
	})
}

// InsertCollection 插入 collection 到 sync_units，使用默认的 nextUSN
// syncCursorUSN 是同步的 USN，注意不是 collection 的 USN
func (i *pullTestInserter) InsertCollection(syncCursorUSN int64) pullTestEntityChange {
	i.t.Helper()
	return i.InsertCollectionWithUSN(i.nextUSN, syncCursorUSN)
}

// InsertCollectionWithUSN records the collection in sync_units with an explicit USN.
// Use an older USN, such as 0, when the collection is setup data before the client's cursor.
func (i *pullTestInserter) InsertCollectionWithUSN(usn, syncCursorUSN int64) pullTestEntityChange {
	i.t.Helper()
	colID := pullTestUUID(i.t)
	now := int64(1_700_000_000_000)
	sqliteSchemaVersion := int64(1)
	lastSyncTime := int64(0)
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO collections (
			user_id, id, usn, sqlite_schema_version, last_sync_time, sync_cursor_usn,
			created_at, updated_at, config
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, JSON_OBJECT())`,
		i.userID,
		colID,
		usn,
		sqliteSchemaVersion,
		lastSyncTime,
		syncCursorUSN,
		now,
		now,
	)
	require.NoError(i.t, err)
	change := pullTestEntityChange{
		entityID:   colID,
		entityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
		usn:        usn,
		updatedAt:  now,
	}
	i.insertSyncUnit(change)
	if i.nextUSN <= usn {
		i.nextUSN = usn + 1
	}
	return change
}

func (i *pullTestInserter) InsertNote(updatedAt int64, fieldsJSON string) pullTestEntityChange {
	i.t.Helper()
	return i.InsertNoteWithUSN(i.nextUSN, updatedAt, fieldsJSON)
}

// InsertNoteWithUSN records the note in sync_units with an explicit USN.
// Use an older USN, such as 0, for dependency notes that should exist but not be pulled.
func (i *pullTestInserter) InsertNoteWithUSN(usn, updatedAt int64, fieldsJSON string) pullTestEntityChange {
	i.t.Helper()
	noteID := pullTestUUID(i.t)
	noteTypeID := pullTestUUID(i.t)
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO notes (
			user_id, id, note_type_id, usn, created_at, updated_at, sense_id, fields, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		i.userID,
		noteID,
		noteTypeID,
		usn,
		1_700_000_000_000,
		updatedAt,
		int32(1),
		fieldsJSON,
	)
	require.NoError(i.t, err)
	change := pullTestEntityChange{
		entityID:   noteID,
		entityType: syncv1.EntityType_ENTITY_TYPE_NOTE,
		usn:        usn,
		updatedAt:  updatedAt,
	}
	i.insertSyncUnit(change)
	if i.nextUSN <= usn {
		i.nextUSN = usn + 1
	}
	return change
}

func (i *pullTestInserter) InsertCard(noteID []byte, updatedAt int64) pullTestEntityChange {
	i.t.Helper()
	cardID := pullTestUUID(i.t)
	deckID := pullTestUUID(i.t)
	usn := i.nextUSN
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO cards (
			user_id, id, note_id, deck_id, usn, updated_at,
			difficulty, stability, scheduled_days, due, last_review,
			lapses, learning_steps, repetitions, state, queue, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		i.userID,
		cardID,
		noteID,
		deckID,
		usn,
		updatedAt,
		2.5,
		3.5,
		1,
		1_700_000_100_000,
		nil,
		0,
		0,
		1,
		1,
		1,
	)
	require.NoError(i.t, err)
	return i.recordChange(pullTestEntityChange{
		entityID:   cardID,
		entityType: syncv1.EntityType_ENTITY_TYPE_CARD,
		usn:        usn,
		updatedAt:  updatedAt,
	})
}

func (i *pullTestInserter) InsertReviewLog(cardID []byte, reviewTime int64) pullTestEntityChange {
	i.t.Helper()
	reviewLogID := pullTestUUID(i.t)
	usn := i.nextUSN
	_, err := suite.Env.DB.ExecContext(
		i.t.Context(),
		`INSERT INTO review_logs (
			user_id, id, card_id, usn, review_time, scheduled_days,
			rating, difficulty, stability, learning_steps, state, duration
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.userID,
		reviewLogID,
		cardID,
		usn,
		reviewTime,
		1,
		3,
		2.6,
		3.6,
		0,
		1,
		30,
	)
	require.NoError(i.t, err)
	return i.recordChange(pullTestEntityChange{
		entityID:   reviewLogID,
		entityType: syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG,
		usn:        usn,
		updatedAt:  reviewTime,
	})
}

func genJSONString(length int) string {
	if length < 2 {
		panic(fmt.Sprintf("invalid json length: %d", length))
	}
	return `"` + genString(length-2) + `"`
}

func genString(length int) string {
	if length < 0 {
		panic(fmt.Sprintf("invalid string length: %d", length))
	}
	return strings.Repeat("a", length)
}

func pullTestUUID(t *testing.T) []byte {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	return id[:]
}
