package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddProcessingNoteChangesStopsWhenLimitReached mock 测试 AddProcessingNoteChanges 在达到 limit 时停止
func TestAddProcessingNoteChangesStopsWhenLimitReached(t *testing.T) {
	rows, cleanup := newProcessingNoteRows(t,
		newProcessingNoteRowArgs(1),
		newProcessingNoteRowArgs(2),
	)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddProcessingNoteChanges(rows, 1)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.False(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
}

// TestAddProcessingNoteChangesStopsWhenCollectorIsFull mock 测试 AddProcessingNoteChanges 在 collector 达到最大大小时停止
func TestAddProcessingNoteChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newProcessingNoteRows(t, newProcessingNoteRowArgs(1))
	defer cleanup()
	c := NewPullCollector()
	c.actualSize = MaxBatchSize

	result, err := c.AddProcessingNoteChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddProcessingNoteChangesReturnsScanError mock 测试 AddProcessingNoteChanges 在 rows.Scan 失败时返回错误
func TestAddProcessingNoteChangesReturnsScanError(t *testing.T) {
	row := newProcessingNoteRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newProcessingNoteRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddProcessingNoteChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newProcessingNoteRows(t *testing.T, processingNoteRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"note_type_id",
		"created_at",
		"updated_at",
		"sense_id",
		"fields",
		"is_deleted",
	})
	for _, row := range processingNoteRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT processing_notes").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT processing_notes")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newProcessingNoteRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("pcs-note-id-0001"),
		usn,
		[]byte("note-type-id-001"),
		int64(1_700_000_000_000),
		int64(1_700_000_000_100),
		int32(1),
		`{"front":"processing"}`,
		false,
	}
}
