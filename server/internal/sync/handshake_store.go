package sync

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type HandshakeStorer interface {
	// GetColInfoForHandshake 仅获取握手需要的集合信息
	//
	// 如果集合不存在，error 为 nil, CollectionInfoForHandshake.SyncCursorUSN 为 1
	//
	// error 不会在集合不存在的时候返回 sql.ErrNoRows，而是返回 nil
	GetColInfoForHandshake(ctx context.Context, userID string) (CollectionInfoForHandshake, error)
}

type HandshakeStore struct {
	db     *sql.DB
	logger logging.Logger
}

func NewHandshakeStore(db *sql.DB, logger logging.Logger) *HandshakeStore {
	return &HandshakeStore{
		db:     db,
		logger: logger,
	}
}

type CollectionInfoForHandshake struct {
	// CollectionID 是集合的唯一标识符
	CollectionID string

	SQLiteSchemaVersion int32
	LastSyncTime        int64

	// SyncCursorUSN 是集合的同步游标 USN
	// 如果服务器集合不存在，则返回 1
	SyncCursorUSN int64

	IsDeleted bool
}

func (s *HandshakeStore) GetColInfoForHandshake(ctx context.Context, userID string) (CollectionInfoForHandshake, error) {
	var info CollectionInfoForHandshake
	var colID []byte
	const sqlStat = `
		SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn, is_deleted
		FROM collections
		WHERE user_id = ?`
	err := s.db.QueryRowContext(
		ctx,
		sqlStat,
		userID,
	).Scan(
		&colID,
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
		return CollectionInfoForHandshake{}, err
	}
	colUUID, err := uuid.FromBytes(colID)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to parse collection id from database", "error", err, "userID", userID, "colID", colID)
		return CollectionInfoForHandshake{}, err
	}
	info.CollectionID = colUUID.String()

	return info, nil
}
