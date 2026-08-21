package sql

import (
	stdsql "database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// 测试 PullStmtCache 在预编译 SQL 语句时出错的情况
func TestPullStmtCacheGetPrepareError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("prepare failed")
	for i, query := range pullOpToSQL {
		expect := mock.ExpectPrepare(regexp.QuoteMeta(query))
		if PullOp(i) == PullOpSelectCards {
			expect.WillReturnError(wantErr)
			break
		}
	}
	stmtCache, err := NewPullStmtCache(t.Context(), db)

	require.Nil(t, stmtCache)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestPullStmtCacheCloseReturnsStatementCloseError 测试 PullStmtCache Close 返回 statement close 错误
func TestPullStmtCacheCloseReturnsStatementCloseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	wantErr := errors.New("close failed")
	mock.ExpectBegin()
	mock.ExpectPrepare("SELECT").WillReturnCloseError(wantErr)
	mock.ExpectRollback()
	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	stmt, err := tx.PrepareContext(t.Context(), "SELECT")
	require.NoError(t, err)
	stmtCache := &PullStmtCache{cache: []*stdsql.Stmt{nil, stmt}}

	err = stmtCache.Close()

	require.ErrorIs(t, err, wantErr)
	require.EqualError(t, err, "failed to close statement: close failed")
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
