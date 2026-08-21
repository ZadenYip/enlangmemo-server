package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type ProcessingNoteRow struct {
	ID         []byte
	Usn        int64
	NoteTypeID sql.Null[[]byte]
	CreatedAt  sql.NullInt64
	UpdatedAt  int64
	SenseID    sql.NullInt32
	Fields     sql.NullString
	IsDeleted  bool
}

const (
	PcsNoteIDSize         = 16
	PcsNoteUsnSize        = 8
	PcsNoteNoteTypeIDSize = 16
	PcsNoteCreatedAtSize  = 8
	PcsNoteUpdatedAtSize  = 8
	PcsNoteSenseIDSize    = 4
	PcsNoteIsDeletedSize  = 1
)

func (c *PullCollector) AddProcessingNoteChanges(rows *sql.Rows, limit int) (CollectResult, error) {
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

		var row ProcessingNoteRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.NoteTypeID,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.SenseID,
			&row.Fields,
			&row.IsDeleted,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = PcsNoteIDSize + PcsNoteNoteTypeIDSize + PcsNoteUsnSize + PcsNoteCreatedAtSize + PcsNoteUpdatedAtSize + PcsNoteSenseIDSize + PcsNoteIsDeletedSize
		const deletedSize = PcsNoteIDSize + PcsNoteUsnSize + PcsNoteUpdatedAtSize + PcsNoteIsDeletedSize

		if row.IsDeleted {
			c.actualSize += deletedSize
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				DeletedAt:  &row.UpdatedAt,
				Usn:        row.Usn,
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		} else {
			c.actualSize += fixedSize + len(row.Fields.String)
			var senseID *int32
			if row.SenseID.Valid {
				senseID = &row.SenseID.Int32
			}
			payload := syncv1.ProcessingNotePayload{
				NoteTypeId: row.NoteTypeID.V,
				CreatedAt:  row.CreatedAt.Int64,
				UpdatedAt:  row.UpdatedAt,
				SenseId:    senseID,
				FieldsJson: row.Fields.String,
			}
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        row.Usn,
				Payload:    &syncv1.SyncChange_ProcessingNote{ProcessingNote: &payload},
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
