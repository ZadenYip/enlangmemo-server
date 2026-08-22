package sync

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"strconv"
	"strings"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type HandshakeStorer interface {
	// GetColInfoForHandshake 仅获取握手需要的集合信息
	//
	// 如果集合不存在，error 为 nil, CollectionInfoForHandshake.SyncCursorUSN 为 1
	//
	// error 不会在集合不存在的时候返回 sql.ErrNoRows，而是返回 nil
	GetColInfoForHandshake(ctx context.Context, userID int64) (CollectionInfoForHandshake, error)

	// 获取 Pulling 状态下要的 PullEntityQueue，确定哪些实体类型需要拉取
	GetPullEntityQueueForHandshake(ctx context.Context, userID int64, minUSNInclusive, maxUSNExclusive int64) (string, error)
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
	CollectionID []byte

	SQLiteSchemaVersion int32
	LastSyncTime        int64

	// SyncCursorUSN 是集合的同步游标 USN
	// 如果服务器集合不存在，则返回 1
	SyncCursorUSN int64
}

func (s *HandshakeStore) GetColInfoForHandshake(ctx context.Context, userID int64) (CollectionInfoForHandshake, error) {
	var info CollectionInfoForHandshake
	const sqlStat = `
			SELECT id, sqlite_schema_version, last_sync_time, sync_cursor_usn
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

	return info, nil
}

//go:embed sql/scripts/select_pull_entity_types.sql
var pullEntityTypesSQL string

func (s *HandshakeStore) GetPullEntityQueueForHandshake(ctx context.Context, userID int64, minUSNInclusive, maxUSNExclusive int64) (string, error) {
	rows, err := s.db.QueryContext(
		ctx,
		pullEntityTypesSQL,
		pullEntityTypesSQLArgs(userID, minUSNInclusive, maxUSNExclusive)...,
	)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get pull entity queue for handshake", "userID", userID, "minUSNInclusive", minUSNInclusive, "maxUSNExclusive", maxUSNExclusive, "error", err)
		return "", err
	}
	defer rows.Close()

	entityTypes := make([]string, 0, 7)
	for rows.Next() {
		var entityType int64
		if err := rows.Scan(&entityType); err != nil {
			s.logger.ErrorCtx(ctx, "failed to scan pull entity type for handshake", "userID", userID, "error", err)
			return "", err
		}
		entityTypes = append(entityTypes, strconv.FormatInt(entityType, 10))
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorCtx(ctx, "failed to iterate pull entity types for handshake", "userID", userID, "error", err)
		return "", err
	}

	queue := strings.Join(entityTypes, ",")
	if queue == "" {
		s.logger.ErrorCtx(ctx, "no entity types to pull for handshake", "userID", userID, "minUSNInclusive", minUSNInclusive, "maxUSNExclusive", maxUSNExclusive)
		return "", errors.New("no entity types to pull for handshake, this should not happen")
	}
	return queue, nil
}

func pullEntityTypesSQLArgs(userID int64, minUSNInclusive, maxUSNExclusive int64) []any {
	args := make([]any, 0, 21)
	for i := 0; i < 7; i++ {
		args = append(args, userID, minUSNInclusive, maxUSNExclusive)
	}
	return args
}
