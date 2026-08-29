package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncsql "github.com/zadenyip/enlangmemo-server/internal/sync/sql"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

var errInvalidPushChange = errors.New("invalid push change")

type PushChangeStorer interface {
	ApplyPushChanges(ctx context.Context, userID int64, assignedStartUSN int64, changes []*syncv1.SyncChange) ([]*syncv1.SyncChange, error)
}

type PushChangeStore struct {
	logger logging.Logger
	db     *sql.DB
}

func NewPushChangeStore(db *sql.DB, logger logging.Logger) *PushChangeStore {
	return &PushChangeStore{
		db:     db,
		logger: logger,
	}
}

func (s *PushChangeStore) ApplyPushChanges(ctx context.Context, userID int64, assignedStartUSN int64, changes []*syncv1.SyncChange) ([]*syncv1.SyncChange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to begin transaction in ApplyPushChanges", "error", err)
		return nil, err
	}
	defer tx.Rollback()
	stmtCache := syncsql.NewPushStmtCache(ctx, tx)
	defer stmtCache.Close()

	assignedChanges := make([]*syncv1.SyncChange, 0, len(changes))
	for i, change := range changes {
		info := applyChangeInfo{
			userID:      userID,
			assignedUSN: assignedStartUSN + int64(i),
		}
		if err := s.applyChange(ctx, info, change, stmtCache); err != nil {
			return nil, err
		}
		assignedChanges = append(assignedChanges, assignedUSNChange(change, info.assignedUSN))
	}
	if err := s.updateColSyncCursor(ctx, tx, userID, assignedStartUSN+int64(len(changes))); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		s.logger.ErrorCtx(ctx, "failed to commit transaction in ApplyPushChanges", "error", err)
		return nil, err
	}

	return assignedChanges, nil
}

func assignedUSNChange(change *syncv1.SyncChange, assignedUSN int64) *syncv1.SyncChange {
	return &syncv1.SyncChange{
		EntityId:   change.GetEntityId(),
		EntityType: change.GetEntityType(),
		Op:         syncv1.ChangeOp_CHANGE_OP_ASSIGN_USN,
		Usn:        assignedUSN,
	}
}

type applyChangeInfo struct {
	userID      int64
	assignedUSN int64
}

func (s *PushChangeStore) updateColSyncCursor(ctx context.Context, tx *sql.Tx, userID int64, nextSyncCursorUSN int64) error {
	result, err := tx.ExecContext(ctx, syncsql.UpdateCollectionSyncCursorSQL(), nextSyncCursorUSN, userID)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to update collection sync cursor", "userID", userID, "syncCursorUSN", nextSyncCursorUSN, "error", err)
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get rows affected after updating collection sync cursor", "userID", userID, "syncCursorUSN", nextSyncCursorUSN, "error", err)
		return err
	}
	if rowsAffected == 0 {
		s.logger.ErrorCtx(ctx, "collection sync cursor update affected no rows", "userID", userID, "syncCursorUSN", nextSyncCursorUSN)
		return errors.New("collection sync cursor update affected no rows")
	}

	return nil
}

func (s *PushChangeStore) applyChange(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	if change == nil {
		return fmt.Errorf("%w: nil sync change", errInvalidPushChange)
	}
	if change.GetUsn() != -1 {
		return fmt.Errorf("%w: sync change usn expected -1, got %d", errInvalidPushChange, change.GetUsn())
	}

	switch change.GetOp() {
	case syncv1.ChangeOp_CHANGE_OP_UPSERT:
		return s.applyUpsert(ctx, info, change, stmtCache)
	case syncv1.ChangeOp_CHANGE_OP_DELETE:
		return s.applyDelete(ctx, info, change, stmtCache)
	default:
		s.logger.ErrorCtx(ctx, "invalid change operation in ApplyPushChanges", "op", change.GetOp())
		return fmt.Errorf("%w: invalid change operation", errInvalidPushChange)
	}
}

func (s *PushChangeStore) applyUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	switch change.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG:
		return s.applyReviewLogUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_CARD:
		return s.applyCardUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_NOTE:
		return s.applyNoteUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE:
		return s.applyProcessingNoteUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE:
		return s.applyNoteTypeUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_DECK:
		return s.applyDeckUpsert(ctx, info, change, stmtCache)
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		return s.applyCollectionUpsert(ctx, info, change, stmtCache)
	default:
		s.logger.ErrorCtx(ctx, "invalid entity type in applyUpsert", "entity_type", change.GetEntityType())
		return fmt.Errorf("%w: invalid entity type", errInvalidPushChange)
	}
}

func (s *PushChangeStore) applyDelete(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	switch change.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG:
		s.logger.ErrorCtx(ctx, "review_log delete is not supported", "entity_id", change.GetEntityId())
		return fmt.Errorf("%w: review_log delete is not supported", errInvalidPushChange)
	case syncv1.EntityType_ENTITY_TYPE_CARD:
		return s.applySoftDelete(ctx, info, stmtCache, syncsql.PushOpDeleteCard, change)
	case syncv1.EntityType_ENTITY_TYPE_NOTE:
		return s.applySoftDelete(ctx, info, stmtCache, syncsql.PushOpDeleteNote, change)
	case syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE:
		return s.applySoftDelete(ctx, info, stmtCache, syncsql.PushOpDeleteProcessingNote, change)
	case syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE:
		return s.applySoftDelete(ctx, info, stmtCache, syncsql.PushOpDeleteNoteType, change)
	case syncv1.EntityType_ENTITY_TYPE_DECK:
		return s.applySoftDelete(ctx, info, stmtCache, syncsql.PushOpDeleteDeck, change)
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		s.logger.ErrorCtx(ctx, "collection delete is not supported", "entity_id", change.GetEntityId())
		return fmt.Errorf("%w: collection delete is not supported", errInvalidPushChange)
	default:
		s.logger.ErrorCtx(ctx, "invalid entity type in applyDelete", "entity_type", change.GetEntityType())
		return fmt.Errorf("%w: invalid entity type", errInvalidPushChange)
	}
}

func (s *PushChangeStore) applyReviewLogUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetReviewLog()
	if payload == nil {
		return fmt.Errorf("%w: missing review_log payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	cardID := payload.GetCardId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertReviewLog)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get upsert review_log statement", "error", err)
		return err
	}

	_, err = stmt.ExecContext(ctx, info.userID, entityID,
		cardID, info.assignedUSN,
		payload.ReviewTime, payload.ScheduledDays,
		payload.Rating, payload.Difficulty, payload.Stability,
		payload.LearningSteps, payload.State, payload.Duration,
	)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to execute query in applyReviewLogChange", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.ReviewTime)
}

func (s *PushChangeStore) applyCardUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetCard()
	if payload == nil {
		return fmt.Errorf("%w: missing card payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	noteID := payload.GetNoteId()
	deckID := payload.GetDeckId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertCard)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID, noteID, deckID, info.assignedUSN,
		payload.UpdatedAt, payload.Difficulty, payload.Stability, payload.ScheduledDays,
		payload.Due, nullableInt64(payload.LastReview), payload.Lapses, payload.LearningSteps,
		payload.Repetitions, payload.State, payload.Queue)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert card", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyNoteUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetNote()
	if payload == nil {
		return fmt.Errorf("%w: missing note payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	noteTypeID := payload.GetNoteTypeId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertNote)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID, noteTypeID,
		info.assignedUSN,
		payload.CreatedAt, payload.UpdatedAt, nullableInt32(payload.SenseId), payload.FieldsJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert note", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyProcessingNoteUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetProcessingNote()
	if payload == nil {
		return fmt.Errorf("%w: missing processing_note payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	noteTypeID := payload.GetNoteTypeId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertProcessingNote)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID, noteTypeID, info.assignedUSN,
		payload.CreatedAt, payload.UpdatedAt, nullableInt32(payload.SenseId), payload.FieldsJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert processing_note", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyNoteTypeUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetNoteType()
	if payload == nil {
		return fmt.Errorf("%w: missing note_type payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertNoteType)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID,
		info.assignedUSN,
		payload.Name, payload.PresetTemplateId, payload.UpdatedAt, payload.NoteTemplateJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert note_type", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyDeckUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetDeck()
	if payload == nil {
		return fmt.Errorf("%w: missing deck payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertDeck)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID,
		info.assignedUSN,
		payload.Name, payload.UpdatedAt,
		payload.NewCardsPerDay, payload.NewLearnedToday,
		payload.LearnedToday, payload.ReviewedToday, payload.ConfigJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert deck", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyCollectionUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *syncsql.PushStmtCache) error {
	payload := change.GetCollection()
	if payload == nil {
		return fmt.Errorf("%w: missing collection payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertCollection)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID,
		info.assignedUSN,
		payload.SqliteSchemaVersion, payload.CreatedAt, payload.UpdatedAt, payload.ConfigJson, false)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert collection", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applySoftDelete(ctx context.Context, info applyChangeInfo, stmtCache *syncsql.PushStmtCache, op syncsql.PushOp, change *syncv1.SyncChange) error {
	if change.DeletedAt == nil {
		return fmt.Errorf("%w: delete missing deleted_at", errInvalidPushChange)
	}
	if change.GetPayload() != nil {
		return fmt.Errorf("%w: delete must not include payload", errInvalidPushChange)
	}

	stmt, err := stmtCache.GetPush(ctx, op)
	if err != nil {
		return err
	}
	deletedAt := change.GetDeletedAt()
	result, err := stmt.ExecContext(ctx, info.assignedUSN, deletedAt, info.userID, change.GetEntityId())
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to apply soft delete", "entity_id", change.GetEntityId(), "entity_type", change.GetEntityType(), "error", err)
		return err
	}

	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.ErrorCtx(ctx, "failed to get rows affected after soft delete", "entity_id", change.GetEntityId(), "entity_type", change.GetEntityType(), "error", err)
		return err
	} else if rowsAffected == 0 {
		// 如果是删除操作，但是没有任何行被影响，说明这个实体已经不存在或者标记删除了，直接跳过即可
		// 之所以客户端要依旧保持会传不存在实体的删除，是因为没法确认另个客户端会不会上传过这个实体，因为修改和创建新实体都是 usn = -1。
		// 即使利用 usn = -2 之类标记是知道上传过服务器，不上传 usn = -1 而是客户端直接物理删除，不产生 tombstone，会遇到客户端因为网络问题没能成功收到分配
		// usn，让客户端以为没上传过这个实体，从而导致同步的时候实体又复活了。
		s.logger.InfoCtx(ctx, "soft delete affected no rows", "entity_id", change.GetEntityId(), "entity_type", change.GetEntityType())
		return nil
	}

	return s.applySyncUnit(ctx, info, stmtCache, change, deletedAt)
}

func (s *PushChangeStore) applySyncUnit(ctx context.Context, info applyChangeInfo, stmtCache *syncsql.PushStmtCache, change *syncv1.SyncChange, updatedAt int64) error {
	entityType := change.GetEntityType()
	op := change.GetOp()

	stmt, err := stmtCache.GetPush(ctx, syncsql.PushOpUpsertSyncUnit)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, change.GetEntityId(), int32(entityType), int32(op), info.assignedUSN, updatedAt)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert sync_unit", "entity_type", entityType, "op", op, "error", err)
		return err
	}

	return nil
}
