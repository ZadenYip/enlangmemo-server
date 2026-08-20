package sql

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// 测试 PushStmtCache 在 预编译 SQL 语句时出错的情况
func TestStmtCacheGetPrepareError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("prepare failed")
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(pushOpToSQL[PushOpUpsertDeck])).WillReturnError(wantErr)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	stmtCache := NewPushStmtCache(t.Context(), tx)

	stmt, err := stmtCache.GetPush(t.Context(), PushOpUpsertDeck)

	require.Nil(t, stmt)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
