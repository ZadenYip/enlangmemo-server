package sync

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestGetCollectionInfoForHandshakeReturnsDefaultValuesWhenCollectionNotFound
// 测试数据库出错时，返回的是空的 CollectionInfoForHandshake{}
func TestGetCollectionInfoForHandshakeReturnsUnderlyingQueryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectQuery("SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn").
		WithArgs(int64(10000)).
		WillReturnError(wantErr)

	store := NewHandshakeStore(db, logging.NewServerLog())

	info, err := store.GetColInfoForHandshake(t.Context(), 10000)

	require.Empty(t, info)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetPullEntityQueueForHandshakeReturnsUnderlyingQueryError
// 测试获取 pull entity queue 的数据库查询出错时返回数据库的原始错误
func TestGetPullEntityQueueForHandshakeReturnsUnderlyingQueryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	args := driverArgs(pullEntityTypesSQLArgs(10000, 1, 8))
	mock.ExpectQuery("SELECT entity_type FROM sync_units").
		WithArgs(args...).
		WillReturnError(wantErr)

	store := NewHandshakeStore(db, logging.NewServerLog())

	queue, err := store.GetPullEntityQueueForHandshake(t.Context(), 10000, 1, 8)

	require.Empty(t, queue)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func driverArgs(args []any) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg
	}
	return values
}

// TestGetCollectionInfoForHandshakeReturnsRawBytes
// 测试数据库返回的集合 ID 会原样保留为 bytes
func TestGetCollectionInfoForHandshakeReturnsRawBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectQuery("SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn").
		WithArgs(int64(10000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"sqlite_schema_version",
			"last_sync_time",
			"sync_cursor_usn",
		}).AddRow([]byte("invalid-uuid-bytes"), 1, int64(2), int64(3)))

	store := NewHandshakeStore(db, logging.NewServerLog())

	info, err := store.GetColInfoForHandshake(t.Context(), 10000)

	require.NoError(t, err)
	require.Equal(t, []byte("invalid-uuid-bytes"), info.CollectionID)
	require.Equal(t, int32(1), info.SQLiteSchemaVersion)
	require.Equal(t, int64(2), info.LastSyncTime)
	require.Equal(t, int64(3), info.SyncCursorUSN)
	require.NoError(t, mock.ExpectationsWereMet())
}
