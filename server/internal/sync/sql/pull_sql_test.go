package sql

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// 测试 Pull StmtCache 在预编译 SQL 语句时出错的情况
func TestPullStmtCacheGetPrepareError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("prepare failed")
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(pullOpToSQL[PullOpSelectCards])).WillReturnError(wantErr)
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer tx.Rollback()
	stmtCache := NewPullStmtCache(t.Context(), tx)

	stmt, err := stmtCache.GetPull(t.Context(), PullOpSelectCards)

	require.Nil(t, stmt)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
