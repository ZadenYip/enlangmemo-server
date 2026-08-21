package sync

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/sync/collector"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

// TestNewPullChangeStoreReturnsStmtCacheError 测试 NewPullChangeStore 在初始化 PullStmtCache 失败时返回错误
func TestNewPullChangeStoreReturnsStmtCacheError(t *testing.T) {
	wantErr := errors.New("prepare failed")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	mock.ExpectPrepare("SELECT").WillReturnError(wantErr)

	store, err := NewPullChangeStore(t.Context(), db, logging.NewServerLog())

	require.Nil(t, store)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestPullChangesReturnsUnsupportedEntityTypeError 测试 GetChangesSinceUSN 在遇到不支持的 entity type 时返回错误
func TestPullChangesReturnsUnsupportedEntityTypeError(t *testing.T) {
	store := &PullChangeStore{logger: logging.NewServerLog()}
	c := collector.NewPullCollector()

	result, err := store.GetChangesSinceUSN(t.Context(), PullInfo{
		UserID:    10000,
		StartUSN:  1,
		EndUSN:    8,
		typeQueue: []int8{127},
	}, c)

	require.Error(t, err)
	require.EqualError(t, err, "unsupported entity type: 127")
	require.Zero(t, result)
}

// TestPullChangesReturnsCollectionQueryError 测试 GetChangesSinceUSN 在 collection 查询失败时返回错误
func TestPullChangesReturnsCollectionQueryError(t *testing.T) {
	wantErr := errors.New("collection query failed")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.MatchExpectationsInOrder(false)
	t.Cleanup(func() {
		_ = db.Close()
	})

	for i := 0; i < 7; i++ {
		prepare := mock.ExpectPrepare("SELECT")
		if i == 0 {
			prepare.ExpectQuery().
				WithArgs(int64(10000), int64(1), int64(8), collector.LimitCol+1).
				WillReturnError(wantErr)
		}
	}

	store, err := NewPullChangeStore(t.Context(), db, logging.NewServerLog())
	require.NoError(t, err)
	c := collector.NewPullCollector()

	result, err := store.GetChangesSinceUSN(t.Context(), PullInfo{
		UserID:    10000,
		StartUSN:  1,
		EndUSN:    8,
		typeQueue: []int8{int8(syncv1.EntityType_ENTITY_TYPE_COLLECTION)},
	}, c)

	require.ErrorIs(t, err, wantErr)
	require.Zero(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}
