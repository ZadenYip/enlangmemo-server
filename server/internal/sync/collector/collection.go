package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type CollectionRow struct {
	ID                  []byte
	Usn                 int64
	SQLiteSchemaVersion sql.NullInt32
	CreatedAt           sql.NullInt64
	UpdatedAt           int64
	Config              sql.NullString
	IsDeleted           bool
}

const (
	collectionIDSize                  = 16
	collectionUsnSize                 = 8
	collectionSQLiteSchemaVersionSize = 4
	collectionCreatedAtSize           = 8
	collectionUpdatedAtSize           = 8
	collectionIsDeletedSize           = 1
)

func (c *PullCollector) AddCollectionChanges(rows *sql.Rows, limit int) (CollectResult, error) {
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

		var row CollectionRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.SQLiteSchemaVersion,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.Config,
			&row.IsDeleted,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = collectionIDSize + collectionUsnSize + collectionSQLiteSchemaVersionSize + collectionCreatedAtSize + collectionUpdatedAtSize + collectionIsDeletedSize
		const deletedSize = collectionIDSize + collectionUsnSize + collectionUpdatedAtSize + collectionIsDeletedSize

		if row.IsDeleted {
			c.actualSize += deletedSize
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				DeletedAt:  &row.UpdatedAt,
				Usn:        row.Usn,
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		} else {
			c.actualSize += fixedSize + len(row.Config.String)
			payload := syncv1.CollectionPayload{
				SqliteSchemaVersion: row.SQLiteSchemaVersion.Int32,
				CreatedAt:           row.CreatedAt.Int64,
				UpdatedAt:           row.UpdatedAt,
				ConfigJson:          row.Config.String,
			}
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_COLLECTION,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        row.Usn,
				Payload:    &syncv1.SyncChange_Collection{Collection: &payload},
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
