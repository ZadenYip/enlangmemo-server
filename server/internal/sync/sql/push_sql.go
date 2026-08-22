package sql

import (
	_ "embed"
)

//go:embed scripts/upsert_reviewlog.sql
var upsertReviewLogSQL string

//go:embed scripts/upsert_card.sql
var upsertCardSQL string

//go:embed scripts/upsert_note.sql
var upsertNoteSQL string

//go:embed scripts/upsert_processing_note.sql
var upsertProcessingNoteSQL string

//go:embed scripts/upsert_note_type.sql
var upsertNoteTypeSQL string

//go:embed scripts/upsert_deck.sql
var upsertDeckSQL string

//go:embed scripts/upsert_collection.sql
var upsertCollectionSQL string

//go:embed scripts/delete_card.sql
var deleteCardSQL string

//go:embed scripts/delete_note.sql
var deleteNoteSQL string

//go:embed scripts/delete_processing_note.sql
var deleteProcessingNoteSQL string

//go:embed scripts/delete_note_type.sql
var deleteNoteTypeSQL string

//go:embed scripts/delete_deck.sql
var deleteDeckSQL string

//go:embed scripts/upsert_sync_unit.sql
var upsertSyncUnitSQL string

//go:embed scripts/update_collection_sync_cursor.sql
var updateCollectionSyncCursorSQL string

type PushOp int

const (
	PushOpUpsertReviewLog PushOp = iota
	PushOpUpsertCard
	PushOpUpsertNote
	PushOpUpsertProcessingNote
	PushOpUpsertNoteType
	PushOpUpsertDeck
	PushOpUpsertCollection

	PushOpDeleteCard
	PushOpDeleteNote
	PushOpDeleteProcessingNote
	PushOpDeleteNoteType
	PushOpDeleteDeck

	PushOpUpsertSyncUnit
)

var pushOpToSQL = [...]string{
	PushOpUpsertReviewLog:      upsertReviewLogSQL,
	PushOpUpsertCard:           upsertCardSQL,
	PushOpUpsertNote:           upsertNoteSQL,
	PushOpUpsertProcessingNote: upsertProcessingNoteSQL,
	PushOpUpsertNoteType:       upsertNoteTypeSQL,
	PushOpUpsertDeck:           upsertDeckSQL,
	PushOpUpsertCollection:     upsertCollectionSQL,

	PushOpDeleteCard:           deleteCardSQL,
	PushOpDeleteNote:           deleteNoteSQL,
	PushOpDeleteProcessingNote: deleteProcessingNoteSQL,
	PushOpDeleteNoteType:       deleteNoteTypeSQL,
	PushOpDeleteDeck:           deleteDeckSQL,

	PushOpUpsertSyncUnit: upsertSyncUnitSQL,
}

func UpdateCollectionSyncCursorSQL() string {
	return updateCollectionSyncCursorSQL
}
