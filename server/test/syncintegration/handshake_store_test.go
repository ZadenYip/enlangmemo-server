package syncintegration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncstore "github.com/zadenyip/enlangmemo-server/internal/sync"
)

func TestHandshakeStoreGetCollectionInfoForHandshake(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	userID := createSyncTestUser(t, "syncuser3")
	store := syncstore.NewHandshakeStore(suite.Env.DB, logging.NewServerLog())

	info, err := store.GetColInfoForHandshake(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, info.CollectionID)
	require.Equal(t, int64(1), info.SyncCursorUSN)
	require.Zero(t, info.SQLiteSchemaVersion)
	require.Zero(t, info.LastSyncTime)
	require.False(t, info.IsDeleted)

	collectionID := insertSyncTestCollection(t, userID, syncTestCollectionRow{
		sqliteSchemaVersion: 15,
		lastSyncTime:        1_800_000_000_000,
		syncCursorUSN:       88,
		isDeleted:           true,
	})

	info, err = store.GetColInfoForHandshake(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, collectionID, info.CollectionID)
	require.Equal(t, int32(15), info.SQLiteSchemaVersion)
	require.Equal(t, int64(1_800_000_000_000), info.LastSyncTime)
	require.Equal(t, int64(88), info.SyncCursorUSN)
	require.True(t, info.IsDeleted)
}

type syncTestCollectionRow struct {
	sqliteSchemaVersion int32
	lastSyncTime        int64
	syncCursorUSN       int64
	isDeleted           bool
}

// loginID 长度不能超过 16 字符
func createSyncTestUser(t *testing.T, loginID string) int64 {
	t.Helper()
	result, err := suite.Env.DB.ExecContext(
		t.Context(),
		`INSERT INTO users (login_id, nickname, password_hash) VALUES (?, ?, ?)`,
		loginID,
		"同步测试用户",
		"integration-test-password-hash",
	)
	require.NoError(t, err)

	id, err := result.LastInsertId()
	require.NoError(t, err)
	return id
}

func insertSyncTestCollection(t *testing.T, userID int64, row syncTestCollectionRow) []byte {
	t.Helper()
	collectionUUID, err := uuid.NewV7()
	require.NoError(t, err)
	now := int64(1_700_000_000_000)

	_, err = suite.Env.DB.ExecContext(
		t.Context(),
		`INSERT INTO collections (
			user_id, id, sqlite_schema_version, last_sync_time, sync_cursor_usn,
			usn, created_at, updated_at, config, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, JSON_OBJECT(), ?)`,
		userID,
		collectionUUID[:],
		row.sqliteSchemaVersion,
		row.lastSyncTime,
		row.syncCursorUSN,
		row.syncCursorUSN,
		now,
		now,
		row.isDeleted,
	)
	require.NoError(t, err)

	return collectionUUID[:]
}
