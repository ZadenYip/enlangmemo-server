package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

const (
	CardIDSize            = 16
	CardUsnSize           = 8
	CardNoteIDSize        = 16
	CardDeckIDSize        = 16
	CardUpdatedAtSize     = 8
	CardDifficultySize    = 8
	CardStabilitySize     = 8
	CardScheduledDaysSize = 4
	CardDueSize           = 8
	CardLastReviewSize    = 8
	CardLapsesSize        = 4
	CardLearningStepsSize = 4
	CardRepetitionsSize   = 4
	CardStateSize         = 4
	CardQueueSize         = 4
	CardIsDeletedSize     = 1
)

type CardRow struct {
	ID            []byte
	Usn           int64
	NoteID        sql.Null[[]byte]
	DeckID        sql.Null[[]byte]
	UpdatedAt     int64
	Difficulty    sql.NullFloat64
	Stability     sql.NullFloat64
	ScheduledDays sql.NullInt32
	Due           sql.NullInt64
	LastReview    sql.NullInt64
	Lapses        sql.NullInt32
	LearningSteps sql.NullInt32
	Repetitions   sql.NullInt32
	State         sql.NullInt32
	Queue         sql.NullInt32
	IsDeleted     bool
}

func (c *PullCollector) AddCardChanges(rows *sql.Rows, limit int) (CollectResult, error) {
	result := CollectResult{}
	count := 0
	for rows.Next() {
		if count == limit {
			result.HasMore = true
			break
		}
		if c.IsFull() {
			result.HasMore = true
			result.SizeExceeded = true
			break
		}

		var row CardRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.NoteID,
			&row.DeckID,
			&row.UpdatedAt,
			&row.Difficulty,
			&row.Stability,
			&row.ScheduledDays,
			&row.Due,
			&row.LastReview,
			&row.Lapses,
			&row.LearningSteps,
			&row.Repetitions,
			&row.State,
			&row.Queue,
			&row.IsDeleted,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = CardIDSize + CardUsnSize + CardNoteIDSize + CardDeckIDSize + CardUpdatedAtSize + CardDifficultySize + CardStabilitySize + CardScheduledDaysSize + CardDueSize + CardLastReviewSize + CardLapsesSize + CardLearningStepsSize + CardRepetitionsSize + CardStateSize + CardQueueSize + CardIsDeletedSize
		const deletedSize = CardIDSize + CardUsnSize + CardUpdatedAtSize + CardIsDeletedSize

		if row.IsDeleted {
			c.actualSize += deletedSize
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_CARD,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				DeletedAt:  &row.UpdatedAt,
				Usn:        row.Usn,
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		} else {
			c.actualSize += fixedSize
			var lastReview *int64
			if row.LastReview.Valid {
				lastReview = &row.LastReview.Int64
			}
			payload := syncv1.CardPayload{
				NoteId:        row.NoteID.V,
				DeckId:        row.DeckID.V,
				UpdatedAt:     row.UpdatedAt,
				Difficulty:    row.Difficulty.Float64,
				Stability:     row.Stability.Float64,
				ScheduledDays: row.ScheduledDays.Int32,
				Due:           row.Due.Int64,
				LastReview:    lastReview,
				Lapses:        row.Lapses.Int32,
				LearningSteps: row.LearningSteps.Int32,
				Repetitions:   row.Repetitions.Int32,
				State:         row.State.Int32,
				Queue:         row.Queue.Int32,
			}
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_CARD,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        row.Usn,
				Payload:    &syncv1.SyncChange_Card{Card: &payload},
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		}

		result.SyncCursorUsn = row.Usn + 1
		c.recordMaxUSN(row.Usn)
		count++
	}
	if err := rows.Err(); err != nil {
		return CollectResult{}, err
	}
	if c.IsFull() {
		result.SizeExceeded = true
	}
	return result, nil
}
