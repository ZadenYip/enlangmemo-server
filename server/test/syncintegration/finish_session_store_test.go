package syncintegration

import (
	"errors"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
)

// TestSessionStoreFinishSyncUpdateLastSyncTimeError 测试 FinishSync 在更新 last_sync_time 失败时返回错误
// 这里的 db 是 mock 的，rdb 是实际的 redis
func TestSessionStoreFinishSyncUpdateLastSyncTimeError(t *testing.T) {
	resetEnv(t)
	ctx := t.Context()
	userID := int64(10001)
	sessionID := "session-id-1"
	finishTime := int64(1_800_000_000_000)
	wantErr := errors.New("update last sync time failed")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	mock.ExpectExec(regexp.QuoteMeta("UPDATE collections SET last_sync_time = ? WHERE user_id = ?;")).
		WithArgs(finishTime, userID).
		WillReturnError(wantErr)
	store := ss.NewSessionStore(db, suite.Env.RDB, logging.NewServerLog())
	session := ss.SyncSession{
		UserID:                      userID,
		State:                       ss.SyncSessionStateAwaitingFinish,
		ExpectedBatchSeq:            1,
		SyncCursorUSN:               12,
		SessionID:                   sessionID,
		CliSyncCursorUSNAtHandshake: 3,
		SrvSyncCursorUSNAtHandshake: 12,
		DeviceID:                    "device-1",
	}

	createResult, err := store.CreateSession(ctx, session)
	require.NoError(t, err)
	require.Equal(t, ss.CreateSessionCreated, createResult)

	err = store.FinishSync(ctx, userID, sessionID, finishTime)

	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	require.NoError(t, mock.ExpectationsWereMet())
	exists, err := suite.Env.RDB.Exists(ctx, ss.RdbSessionKey(userID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}
