package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddCardChangesStopsWhenLimitReached mock 测试 AddCardChanges 在达到 limit 时停止
func TestAddCardChangesStopsWhenLimitReached(t *testing.T) {
	rows, cleanup := newCardRows(t,
		newCardRowArgs(1),
		newCardRowArgs(2),
	)
	defer cleanup()
	c := NewPullCollector()

	// limit 设置为 1，确保在处理第二个 row 时停止
	result, err := c.AddCardChanges(rows, 1)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.False(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_CARD, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
}

// TestAddCardChangesStopsWhenCollectorIsFull mock 测试 AddCardChanges 在 collector 达到最大大小时停止
func TestAddCardChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newCardRows(t, newCardRowArgs(1))
	defer cleanup()
	c := NewPullCollector()

	// 设置 actualSize 为 MaxBatchSize，确保在处理第一个 row 时停止
	c.actualSize = MaxBatchSize

	result, err := c.AddCardChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddCardChangesReturnsScanError mock 测试 AddCardChanges 在 rows.Scan 失败时返回错误
func TestAddCardChangesReturnsScanError(t *testing.T) {
	row := newCardRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newCardRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddCardChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newCardRows(t *testing.T, cardRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"note_id",
		"deck_id",
		"updated_at",
		"difficulty",
		"stability",
		"scheduled_days",
		"due",
		"last_review",
		"lapses",
		"learning_steps",
		"repetitions",
		"state",
		"queue",
		"is_deleted",
	})
	for _, row := range cardRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT cards").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT cards")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newCardRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("card-id-00000001"),
		usn,
		[]byte("note-id-00000001"),
		[]byte("deck-id-00000001"),
		int64(1_700_000_000_000),
		2.5,
		3.5,
		int32(1),
		int64(1_700_000_100_000),
		int64(1_700_000_050_000),
		int32(0),
		int32(0),
		int32(1),
		int32(1),
		int32(1),
		false,
	}
}
