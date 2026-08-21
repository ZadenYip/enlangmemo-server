package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type CollectionRow struct {
	ID                  []byte
	Usn                 int64
	SQLiteSchemaVersion int32
	CreatedAt           int64
	UpdatedAt           int64
	Config              string
}

const (
	ColIDSize                  = 16
	ColUsnSize                 = 8
	ColSQLiteSchemaVersionSize = 4
	ColCreatedAtSize           = 8
	ColUpdatedAtSize           = 8
)

func (c *PullCollector) AddCollectionChanges(rows *sql.Rows, limit int) (CollectResult, error) {
	result := CollectResult{}
	count := 0
	for rows.Next() {
		// if count == limit {
		// 	result.HasMore = true
		// 	break
		// }
		if c.IsFull() {
			result.HasMore = true
			result.SizeExceeded = true
			break
		}

		var row CollectionRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.SQLiteSchemaVersion,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.Config,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = ColIDSize + ColUsnSize + ColSQLiteSchemaVersionSize + ColCreatedAtSize + ColUpdatedAtSize

		c.actualSize += fixedSize + len(row.Config)
		payload := syncv1.CollectionPayload{
			SqliteSchemaVersion: row.SQLiteSchemaVersion,
			CreatedAt:           row.CreatedAt,
			UpdatedAt:           row.UpdatedAt,
			ConfigJson:          row.Config,
		}
		syncChange := syncv1.SyncChange{
			EntityId:   row.ID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        row.Usn,
			Payload:    &syncv1.SyncChange_Collection{Collection: &payload},
		}
		c.syncChanges = append(c.syncChanges, &syncChange)

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
