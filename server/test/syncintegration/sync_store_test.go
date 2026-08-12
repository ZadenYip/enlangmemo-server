package syncintegration

import (
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncstore "github.com/zadenyip/enlangmemo-server/internal/sync"
)

func TestCollectionStoreGetColInfoForHandshake(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	userID := createSyncTestUser(t, "syncuser3")
	store := syncstore.NewCollectionStore(suite.Env.DB, logging.NewServerLog())

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

func TestSessionStoreCreateSession(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	store := syncstore.NewSessionStore(suite.Env.RDB, logging.NewServerLog())
	session := syncstore.SyncSession{
		UserID:                      "session-user-1",
		State:                       syncstore.SyncSessionStatePulling,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   "session-id-1",
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	result, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionCreated, result)

	key := "sync:" + session.UserID + ":sync_lock"
	got, err := suite.Env.RDB.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"user_id":                             session.UserID,
		"state":                               strconv.FormatInt(int64(session.State), 10),
		"expected_batch_seq":                  "1",
		"sync_cursor_usn":                     "12",
		"session_id":                          session.SessionID,
		"client_sync_cursor_usn_at_handshake": "3",
		"server_sync_cursor_usn_at_handshake": "12",
		"device_id":                           session.DeviceID,
	}, got)

	ttl, err := suite.Env.RDB.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl.Seconds(), float64(60))

	result, err = store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, syncstore.CreateSessionAlreadyExists, result)
}

type syncTestCollectionRow struct {
	sqliteSchemaVersion int32
	lastSyncTime        int64
	syncCursorUSN       int64
	isDeleted           bool
}

func createSyncTestUser(t *testing.T, loginID string) string {
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
	return strconv.FormatInt(id, 10)
}

func insertSyncTestCollection(t *testing.T, userID string, row syncTestCollectionRow) string {
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

	return collectionUUID.String()
}
