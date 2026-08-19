package sync

import (
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

	mock.ExpectQuery("SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn, is_deleted").
		WithArgs(int64(10000)).
		WillReturnError(wantErr)

	store := NewHandshakeStore(db, logging.NewServerLog())

	info, err := store.GetColInfoForHandshake(t.Context(), 10000)

	require.Empty(t, info)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetCollectionInfoForHandshakeReturnsRawBytes
// 测试数据库返回的集合 ID 会原样保留为 bytes
func TestGetCollectionInfoForHandshakeReturnsRawBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	mock.ExpectQuery("SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn, is_deleted").
		WithArgs(int64(10000)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"sqlite_schema_version",
			"last_sync_time",
			"sync_cursor_usn",
			"is_deleted",
		}).AddRow([]byte("invalid-uuid-bytes"), 1, int64(2), int64(3), false))

	store := NewHandshakeStore(db, logging.NewServerLog())

	info, err := store.GetColInfoForHandshake(t.Context(), 10000)

	require.NoError(t, err)
	require.Equal(t, []byte("invalid-uuid-bytes"), info.CollectionID)
	require.Equal(t, int32(1), info.SQLiteSchemaVersion)
	require.Equal(t, int64(2), info.LastSyncTime)
	require.Equal(t, int64(3), info.SyncCursorUSN)
	require.False(t, info.IsDeleted)
	require.NoError(t, mock.ExpectationsWereMet())
}
