package collector

import (
	"database/sql"

	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type NoteTypeRow struct {
	ID               []byte
	Usn              int64
	Name             sql.NullString
	PresetTemplateID sql.NullInt32
	UpdatedAt        int64
	NoteTemplate     sql.NullString
	IsDeleted        bool
}

const (
	NoteTypeIDSize               = 16
	NoteTypeUsnSize              = 8
	NoteTypePresetTemplateIDSize = 4
	NoteTypeUpdatedAtSize        = 8
	NoteTypeIsDeletedSize        = 1
)

func (c *PullCollector) AddNoteTypeChanges(rows *sql.Rows, limit int) (CollectResult, error) {
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

		var row NoteTypeRow
		err := rows.Scan(
			&row.ID,
			&row.Usn,
			&row.Name,
			&row.PresetTemplateID,
			&row.UpdatedAt,
			&row.NoteTemplate,
			&row.IsDeleted,
		)
		if err != nil {
			return CollectResult{}, err
		}

		const fixedSize = NoteTypeIDSize + NoteTypeUsnSize + NoteTypePresetTemplateIDSize + NoteTypeUpdatedAtSize + NoteTypeIsDeletedSize
		const deletedSize = NoteTypeIDSize + NoteTypeUsnSize + NoteTypeUpdatedAtSize + NoteTypeIsDeletedSize

		if row.IsDeleted {
			c.actualSize += deletedSize
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE,
				Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
				DeletedAt:  &row.UpdatedAt,
				Usn:        row.Usn,
			}
			c.syncChanges = append(c.syncChanges, &syncChange)
		} else {
			c.actualSize += fixedSize + len(row.Name.String) + len(row.NoteTemplate.String)
			payload := syncv1.NoteTypePayload{
				Name:             row.Name.String,
				PresetTemplateId: row.PresetTemplateID.Int32,
				UpdatedAt:        row.UpdatedAt,
				NoteTemplateJson: row.NoteTemplate.String,
			}
			syncChange := syncv1.SyncChange{
				EntityId:   row.ID,
				EntityType: syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE,
				Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
				Usn:        row.Usn,
				Payload:    &syncv1.SyncChange_NoteType{NoteType: &payload},
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
