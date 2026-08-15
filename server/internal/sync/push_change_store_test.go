package sync

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestApplyPushChangesBeginTxError 测试 ApplyPushChanges 在打开事务过程数据库出错的情况
func TestApplyPushChangesBeginTxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("begin tx failed")
	mock.ExpectBegin().WillReturnError(wantErr)
	store := NewPushChangeStore(db, logging.NewServerLog())

	err = store.ApplyPushChanges(t.Context(), "10001", 12, nil)

	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestApplyPushChangesCommitTxError 测试 ApplyPushChanges 在向事务执行 updateCollectionSyncCursorSQL 语句时出错的情况
func TestUpdateColSyncCursorExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("update failed")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateCollectionSyncCursorSQL)).
		WithArgs(int64(13), "10001").
		WillReturnError(wantErr)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	store := NewPushChangeStore(db, logging.NewServerLog())

	err = store.updateColSyncCursor(t.Context(), tx, applyChangeInfo{userID: "10001", assignedUSN: 12})

	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateColSyncCursorRowsAffectedError 测试 updateColSyncCursor 在检查受影响行数出错的情况
func TestUpdateColSyncCursorRowsAffectedError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("rows affected failed")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateCollectionSyncCursorSQL)).
		WithArgs(int64(13), "10001").
		WillReturnResult(sqlmock.NewErrorResult(wantErr))
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	store := NewPushChangeStore(db, logging.NewServerLog())

	err = store.updateColSyncCursor(t.Context(), tx, applyChangeInfo{userID: "10001", assignedUSN: 12})

	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateColSyncCursorNoRowsAffected 测试 updateColSyncCursor 在受影响行数为 0 的情况
func TestUpdateColSyncCursorNoRowsAffected(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateCollectionSyncCursorSQL)).
		WithArgs(int64(13), "10001").
		WillReturnResult(sqlmock.NewResult(0, 0))
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	store := NewPushChangeStore(db, logging.NewServerLog())

	err = store.updateColSyncCursor(t.Context(), tx, applyChangeInfo{userID: "10001", assignedUSN: 12})

	require.EqualError(t, err, "collection sync cursor update affected no rows")
	require.NoError(t, mock.ExpectationsWereMet())
}
