package sync

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type CollectionStorer interface {
	GetServerSyncCursorUSN(ctx context.Context, userID string, colID string) (int64, error)
	// GetColInfoForHandshake 仅获取握手需要的集合信息
	//
	// 如果集合不存在，error 为 nil, ColInfoForHandshake.SyncCursorUSN 为 1
	//
	// error 不会在集合不存在的时候返回 sql.ErrNoRows，而是返回 nil
	GetColInfoForHandshake(ctx context.Context, userID string) (ColInfoForHandshake, error)
}

type CollectionStore struct {
	db     *sql.DB
	logger logging.Logger
}

func NewCollectionStore(db *sql.DB, logger logging.Logger) *CollectionStore {
	return &CollectionStore{
		db:     db,
		logger: logger,
	}
}

func (s *CollectionStore) GetServerSyncCursorUSN(ctx context.Context, userID string, colID string) (int64, error) {
	var usn int64 = 0
	const sqlStat = `SELECT sync_cursor_usn FROM collections WHERE user_id = ? AND id = ?`
	err := s.db.QueryRowContext(
		ctx,
		sqlStat,
		userID, colID,
	).Scan(&usn)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		s.logger.InfoCtx(ctx, "no collection found", "userID", userID, "collectionID", colID)
		usn = 0
		return usn, nil
	case err != nil:
		s.logger.ErrorCtx(ctx, "failed to get server sync cursor usn", "error", err)
		return 0, err
	}

	return usn, nil
}

type ColInfoForHandshake struct {
	// CollectionID 是集合的唯一标识符
	CollectionID string

	SQLiteSchemaVersion int32
	LastSyncTime        int64

	// SyncCursorUSN 是集合的同步游标 USN
	// 如果服务器集合不存在，则返回 1
	SyncCursorUSN int64

	IsDeleted bool
}

func (s *CollectionStore) GetColInfoForHandshake(ctx context.Context, userID string) (ColInfoForHandshake, error) {
	var info ColInfoForHandshake
	const sqlStat = `
		SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn, is_deleted
		FROM collections
		WHERE user_id = ?`
	err := s.db.QueryRowContext(
		ctx,
		sqlStat,
		userID,
	).Scan(
		&info.CollectionID,
		&info.SQLiteSchemaVersion,
		&info.LastSyncTime,
		&info.SyncCursorUSN,
		&info.IsDeleted,
	)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		s.logger.InfoCtx(ctx, "no collection found for handshake", "userID", userID)
		info.SyncCursorUSN = 1
		return info, nil
	case err != nil:
		s.logger.ErrorCtx(ctx, "failed to get collection info for handshake", "error", err)
		return ColInfoForHandshake{}, err
	}

	return info, nil
}
