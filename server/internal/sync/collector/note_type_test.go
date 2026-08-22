package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddNoteTypeChangesStopsWhenLimitReached mock 测试 AddNoteTypeChanges 在达到 limit 时停止
func TestAddNoteTypeChangesStopsWhenLimitReached(t *testing.T) {
	rows, cleanup := newNoteTypeRows(t,
		newNoteTypeRowArgs(1),
		newNoteTypeRowArgs(2),
	)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddNoteTypeChanges(rows, 1)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.False(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
}

// TestAddNoteTypeChangesStopsWhenCollectorIsFull mock 测试 AddNoteTypeChanges 在 collector 达到最大大小时停止
func TestAddNoteTypeChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newNoteTypeRows(t, newNoteTypeRowArgs(1))
	defer cleanup()
	c := NewPullCollector()
	c.actualSize = MaxBatchSize

	result, err := c.AddNoteTypeChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddNoteTypeChangesReturnsScanError mock 测试 AddNoteTypeChanges 在 rows.Scan 失败时返回错误
func TestAddNoteTypeChangesReturnsScanError(t *testing.T) {
	row := newNoteTypeRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newNoteTypeRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddNoteTypeChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newNoteTypeRows(t *testing.T, noteTypeRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"name",
		"preset_template_id",
		"updated_at",
		"note_template",
		"is_deleted",
	})
	for _, row := range noteTypeRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT note_types").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT note_types")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newNoteTypeRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("note-type-id-001"),
		usn,
		"basic",
		int32(1),
		int64(1_700_000_000_000),
		`{"template":true}`,
		false,
	}
}
