package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestAddNoteChangesReturnsScanError mock 测试 AddNoteChanges 在 rows.Scan 失败时返回错误
func TestAddNoteChangesReturnsScanError(t *testing.T) {
	row := newNoteRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newNoteRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddNoteChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newNoteRows(t *testing.T, noteRows ...[]driver.Value) (*sql.Rows, func()) {
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
	for _, row := range noteRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT notes").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT notes")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newNoteRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("note-id-00000001"),
		usn,
		[]byte("note-type-id-001"),
		int64(1_700_000_000_000),
		int64(1_700_000_000_100),
		int32(1),
		`{"front":"note"}`,
		false,
	}
}
