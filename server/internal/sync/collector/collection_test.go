package collector

import (
	"database/sql"
	"database/sql/driver"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestAddCollectionChangesStopsWhenCollectorIsFull mock 测试 AddCollectionChanges 在 collector 已达到最大大小时停止
func TestAddCollectionChangesStopsWhenCollectorIsFull(t *testing.T) {
	rows, cleanup := newCollectionRows(t, newCollectionRowArgs(1, `{"collection":true}`))
	defer cleanup()
	c := NewPullCollector()
	c.actualSize = MaxBatchSize

	result, err := c.AddCollectionChanges(rows, 10)

	require.NoError(t, err)
	require.True(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Zero(t, result.SyncCursorUsn)
	require.Empty(t, c.Changes())
}

// TestAddCollectionChangesMarksSizeExceededAfterCollectionFillsBatch mock 测试添加完 collection 后刚好达到 batch 最大大小
func TestAddCollectionChangesMarksSizeExceededAfterCollectionFillsBatch(t *testing.T) {
	const fixedSize = ColIDSize + ColUsnSize + ColSQLiteSchemaVersionSize + ColCreatedAtSize + ColUpdatedAtSize
	config := strings.Repeat("x", MaxBatchSize-fixedSize)
	rows, cleanup := newCollectionRows(t, newCollectionRowArgs(1, config))
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddCollectionChanges(rows, 10)

	require.NoError(t, err)
	require.False(t, result.HasMore)
	require.True(t, result.SizeExceeded)
	require.Equal(t, int64(2), result.SyncCursorUsn)
	require.Len(t, c.Changes(), 1)
	require.Equal(t, syncv1.EntityType_ENTITY_TYPE_COLLECTION, c.Changes()[0].GetEntityType())
	require.Equal(t, int64(1), c.Changes()[0].GetUsn())
	require.Equal(t, config, c.Changes()[0].GetCollection().GetConfigJson())
}

// TestAddCollectionChangesReturnsScanError mock 测试 AddCollectionChanges 在 rows.Scan 失败时返回错误
func TestAddCollectionChangesReturnsScanError(t *testing.T) {
	row := newCollectionRowArgs(1, `{"collection":true}`)
	row[1] = "invalid-usn"
	rows, cleanup := newCollectionRows(t, row)
	defer cleanup()
	c := NewPullCollector()

	result, err := c.AddCollectionChanges(rows, 10)

	require.Error(t, err)
	require.Zero(t, result)
	require.Empty(t, c.Changes())
}

func newCollectionRows(t *testing.T, collectionRows ...[]driver.Value) (*sql.Rows, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{
		"id",
		"usn",
		"sqlite_schema_version",
		"created_at",
		"updated_at",
		"config",
	})
	for _, row := range collectionRows {
		rows.AddRow(row...)
	}
	mock.ExpectQuery("SELECT collections").WillReturnRows(rows)
	mock.ExpectClose()

	sqlRows, err := db.Query("SELECT collections")
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, sqlRows.Close())
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return sqlRows, cleanup
}

func newCollectionRowArgs(usn int64, config string) []driver.Value {
	return []driver.Value{
		[]byte("collection-id-01"),
		usn,
		int32(1),
		int64(1_700_000_000_000),
		int64(1_700_000_000_100),
		config,
	}
}
