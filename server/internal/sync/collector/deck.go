package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type DeckRow struct {
	ID              []byte
	Usn             int64
	Name            sql.NullString
	UpdatedAt       int64
	NewCardsPerDay  sql.NullInt32
	NewLearnedToday sql.NullInt32
	LearnedToday    sql.NullInt32
	ReviewedToday   sql.NullInt32
	Config          sql.NullString
	IsDeleted       bool
}

const (
	deckIDSize              = 16
	deckUsnSize             = 8
	deckUpdatedAtSize       = 8
	deckNewCardsPerDaySize  = 4
	deckNewLearnedTodaySize = 4
	deckLearnedTodaySize    = 4
	deckReviewedTodaySize   = 4
	deckIsDeletedSize       = 1
)

func (c *PullCollector) AddDeckChanges(rows *sql.Rows, limit int) (CollectResult, error) {
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

		var row DeckRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.Name,
			&row.UpdatedAt,
			&row.NewCardsPerDay,
			&row.NewLearnedToday,
			&row.LearnedToday,
			&row.ReviewedToday,
			&row.Config,
			&row.IsDeleted,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = deckIDSize + deckUsnSize + deckUpdatedAtSize + deckNewCardsPerDaySize + deckNewLearnedTodaySize + deckLearnedTodaySize + deckReviewedTodaySize + deckIsDeletedSize
		const deletedSize = deckIDSize + deckUsnSize + deckUpdatedAtSize + deckIsDeletedSize

		if row.IsDeleted {
			c.actualSize += deletedSize
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				DeletedAt:  &row.UpdatedAt,
				Usn:        row.Usn,
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		} else {
			c.actualSize += fixedSize + len(row.Name.String) + len(row.Config.String)
			payload := syncv1.DeckPayload{
				Name:            row.Name.String,
				UpdatedAt:       row.UpdatedAt,
				NewCardsPerDay:  row.NewCardsPerDay.Int32,
				NewLearnedToday: row.NewLearnedToday.Int32,
				LearnedToday:    row.LearnedToday.Int32,
				ReviewedToday:   row.ReviewedToday.Int32,
				ConfigJson:      row.Config.String,
			}
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        row.Usn,
				Payload:    &syncv1.SyncChange_Deck{Deck: &payload},
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		}

		result.SyncCursorUsn = row.Usn + 1
		c.maxUSN = max(result.SyncCursorUsn, c.maxUSN)
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
