package sync

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// 测试 StmtCache 在 预编译 SQL 语句时出错的情况
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
	stmtCache := NewStmtCache(t.Context(), tx)

	stmt, err := stmtCache.Get(t.Context(), PushOpUpsertDeck)

	require.Nil(t, stmt)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
