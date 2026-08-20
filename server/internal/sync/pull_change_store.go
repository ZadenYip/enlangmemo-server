package sync

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zadenyip/enlangmemo-server/internal/sync/collector"
	ssql "github.com/zadenyip/enlangmemo-server/internal/sync/sql"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type PullChangeStorer interface {
	// GetChangesSinceUSN 获取自指定 USN 之后的所有变更
	GetChangesSinceUSN(ctx context.Context, info PullInfo, c *collector.PullCollector) (PullChangesResult, error)
}

type PullChangeStore struct {
	db        *sql.DB
	logger    logging.Logger
	stmtCache *ssql.PullStmtCache
}

func NewPullChangeStore(ctx context.Context, db *sql.DB, logger logging.Logger) (*PullChangeStore, error) {
	stmtCache, err := ssql.NewPullStmtCache(ctx, db)
	if err != nil {
		return nil, err
	}
	return &PullChangeStore{
		db:        db,
		logger:    logger,
		stmtCache: stmtCache,
	}, nil
}

func (s *PullChangeStore) GracefulShutdown() error {
	return s.stmtCache.Close()
}

type PullInfo struct {
	UserID int64
	// StartUSN 指定的起始 USN
	StartUSN int64
	// EndUSN 指定的结束 USN
	EndUSN    int64
	typeQueue []int8
}

type PullChangesResult struct {
	typeQueue []int8
	// SyncCursorUSN 表示当前同步游标的 USN
	SyncCursorUSN int64
}

func (s *PullChangeStore) GetChangesSinceUSN(ctx context.Context, info PullInfo, c *collector.PullCollector) (PullChangesResult, error) {
	var typeQueue []int8 = info.typeQueue
	var startUSN int64 = info.StartUSN
	var cursorUSN int64 = info.StartUSN
	var endUSN int64 = info.EndUSN

	for len(typeQueue) > 0 {
		entityType := typeQueue[0]
		colResult := collector.CollectResult{}
		var err error

		switch entityType {
		case int8(syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG):
			colResult, err = s.PullReviewLogs(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_CARD):
			colResult, err = s.PullCards(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_NOTE):
			colResult, err = s.PullNotes(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE):
			colResult, err = s.PullProcessingNotes(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE):
			colResult, err = s.PullNoteTypes(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_DECK):
			colResult, err = s.PullDecks(ctx, info.UserID, cursorUSN, endUSN, c)
		case int8(syncv1.EntityType_ENTITY_TYPE_COLLECTION):
			colResult, err = s.PullCollection(ctx, info.UserID, cursorUSN, endUSN, c)
		default:
			s.logger.ErrorCtx(ctx, "unsupported entity type in Pull", "userID", info.UserID, "entityType", entityType)
			return PullChangesResult{}, fmt.Errorf("unsupported entity type: %d", entityType)
		}
		if err != nil {
			return PullChangesResult{}, err
		}
		// SyncCursorUsn 小于等于 cursorUSN，理论上不应该发送，如果发生是逻辑错误
		if colResult.SyncCursorUsn <= cursorUSN {
			s.logger.ErrorCtx(ctx, "invalid SyncCursorUsn", "userID", info.UserID, "entityType", entityType, "SyncCursorUsn", colResult.SyncCursorUsn, "cursorUSN", cursorUSN)
			return PullChangesResult{}, fmt.Errorf("invalid SyncCursorUsn: %d, expected greater than %d", colResult.SyncCursorUsn, cursorUSN)
		}

		if colResult.SizeExceeded {
			if colResult.HasMore {
				cursorUSN = colResult.SyncCursorUsn
				return PullChangesResult{typeQueue: typeQueue, SyncCursorUSN: cursorUSN}, nil
			} else {
				// 这个实体类型拉取完了，SyncCursorUSN 返回 startUSN
				return PullChangesResult{typeQueue: typeQueue[1:], SyncCursorUSN: startUSN}, nil
			}
		} else {
			if colResult.HasMore {
				cursorUSN = colResult.SyncCursorUsn
				continue
			} else {
				// 这个实体类型拉取完了，重置 cursorUSN 回 startUSN
				cursorUSN = startUSN

				typeQueue = typeQueue[1:]
			}
		}
	}

	return PullChangesResult{typeQueue: typeQueue, SyncCursorUSN: endUSN}, nil
}

func (s *PullChangeStore) PullCollection(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectCollection)
	limit := collector.LimitCol
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query collection", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddCollectionChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add collection changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullDecks(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectDecks)
	limit := collector.LimitDeck
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query decks", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddDeckChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add deck changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullNoteTypes(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectNoteTypes)
	limit := collector.LimitNoteType
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query note types", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddNoteTypeChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add note type changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullNotes(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectNotes)
	limit := collector.LimitNote
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query notes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddNoteChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add note changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullProcessingNotes(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectProcessingNotes)
	limit := collector.LimitProcessingNote
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query processing notes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddProcessingNoteChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add processing note changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullCards(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOpSelectCards)
	limit := collector.LimitCard
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query cards", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddCardChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add card changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}

func (s *PullChangeStore) PullReviewLogs(ctx context.Context, userID int64, startUSN, endUSN int64, c *collector.PullCollector) (collector.CollectResult, error) {
	stmt := s.stmtCache.GetPull(ctx, ssql.PullOp(ssql.PullOpSelectReviewLogs))
	limit := collector.LimitReviewLog
	rows, err := stmt.QueryContext(ctx, userID, startUSN, endUSN, limit+1)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to query review logs", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}
	defer rows.Close()

	result, err := c.AddReviewLogChanges(rows, limit)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to add review log changes", "userID", userID, "error", err)
		return collector.CollectResult{}, err
	}

	return result, nil
}
