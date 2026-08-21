package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddDeckChangesStopsWhenLimitReached mock 测试 AddDeckChanges 在达到 limit 时停止
func TestAddDeckChangesStopsWhenLimitReached(t *testing.T) {
	rows, cleanup := newDeckRows(t,
		newDeckRowArgs(1),
		newDeckRowArgs(2),
	)
	defer cleanup()
	c := NewPullCollector()

	// limit 设置为 1，确保在处理第二个 row 时停止
	result, err := c.AddDeckChanges(rows, 1)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.False(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_DECK, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
}

// TestAddDeckChangesStopsWhenCollectorIsFull mock 测试 AddDeckChanges 在 collector 达到最大大小时停止
func TestAddDeckChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newDeckRows(t, newDeckRowArgs(1))
	defer cleanup()
	c := NewPullCollector()

	// 设置 actualSize 为 MaxBatchSize，确保在处理第一个 row 时停止
	c.actualSize = MaxBatchSize

	result, err := c.AddDeckChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddDeckChangesReturnsScanError mock 测试 AddDeckChanges 在 rows.Scan 失败时返回错误
func TestAddDeckChangesReturnsScanError(t *testing.T) {
	row := newDeckRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newDeckRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddDeckChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newDeckRows(t *testing.T, deckRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"name",
		"updated_at",
		"new_cards_per_day",
		"new_learned_today",
		"learned_today",
		"reviewed_today",
		"config",
		"is_deleted",
	})
	for _, row := range deckRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT decks").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT decks")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newDeckRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("deck-id-00000001"),
		usn,
		"deck",
		int64(1_700_000_000_000),
		int32(20),
		int32(1),
		int32(2),
		int32(3),
		`{"deck":true}`,
		false,
	}
}
