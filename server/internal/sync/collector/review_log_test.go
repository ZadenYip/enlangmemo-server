package collector

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddReviewLogChangesStopsWhenLimitReached mock 测试 AddReviewLogChanges 在达到 limit 时停止
func TestAddReviewLogChangesStopsWhenLimitReached(t *testing.T) {
	rows, cleanup := newReviewLogRows(t,
		newReviewLogRowArgs(1),
		newReviewLogRowArgs(2),
	)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddReviewLogChanges(rows, 1)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.False(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
}

// TestAddReviewLogChangesStopsWhenCollectorIsFull mock 测试 AddReviewLogChanges 在 collector 达到最大大小时停止
func TestAddReviewLogChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newReviewLogRows(t, newReviewLogRowArgs(1))
	defer cleanup()
	c := NewPullCollector()
	c.actualSize = MaxBatchSize

	result, err := c.AddReviewLogChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddReviewLogChangesReturnsScanError mock 测试 AddReviewLogChanges 在 rows.Scan 失败时返回错误
func TestAddReviewLogChangesReturnsScanError(t *testing.T) {
	row := newReviewLogRowArgs(1)
	row[1] = "invalid-usn"
	rows, cleanup := newReviewLogRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddReviewLogChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newReviewLogRows(t *testing.T, reviewLogRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"card_id",
		"review_time",
		"scheduled_days",
		"rating",
		"difficulty",
		"stability",
		"learning_steps",
		"state",
		"duration",
	})
	for _, row := range reviewLogRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT review_logs").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT review_logs")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newReviewLogRowArgs(usn int64) []driver.Value {
	return []driver.Value{
		[]byte("review-log-id-01"),
		usn,
		[]byte("card-id-00000001"),
		int64(1_700_000_000_000),
		int32(1),
		int32(3),
		2.6,
		3.6,
		int32(0),
		int32(1),
		int32(30),
	}
}
