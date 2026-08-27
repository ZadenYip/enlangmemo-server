package sql

import _ "embed"

//go:embed scripts/pull_collection.sql
var pullCollectionSQL string

//go:embed scripts/pull_decks.sql
var pullDecksSQL string

//go:embed scripts/pull_note_types.sql
var pullNoteTypesSQL string

//go:embed scripts/pull_processing_notes.sql
var pullProcessingNotesSQL string

//go:embed scripts/pull_notes.sql
var pullNotesSQL string

//go:embed scripts/pull_cards.sql
var pullCardsSQL string

//go:embed scripts/pull_review_logs.sql
var pullReviewLogsSQL string

type PullOp int

const (
	PullOpSelectCollection PullOp = iota
	PullOpSelectDecks
	PullOpSelectNoteTypes
	PullOpSelectProcessingNotes
	PullOpSelectNotes
	PullOpSelectCards
	PullOpSelectReviewLogs
)

var pullOpToSQL = [...]string{
	PullOpSelectCollection:      pullCollectionSQL,
	PullOpSelectDecks:           pullDecksSQL,
	PullOpSelectNoteTypes:       pullNoteTypesSQL,
	PullOpSelectProcessingNotes: pullProcessingNotesSQL,
	PullOpSelectNotes:           pullNotesSQL,
	PullOpSelectCards:           pullCardsSQL,
	PullOpSelectReviewLogs:      pullReviewLogsSQL,
}
