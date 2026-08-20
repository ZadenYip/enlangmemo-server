package sql

import (
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
