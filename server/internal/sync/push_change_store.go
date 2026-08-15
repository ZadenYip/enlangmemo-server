package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

var errInvalidPushChange = errors.New("invalid push change")

type PushChangeStorer interface {
	ApplyPushChanges(ctx context.Context, userID string, assignedUSN int64, changes []*syncv1.SyncChange) error
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

func (s *PushChangeStore) ApplyPushChanges(ctx context.Context, userID string, assignedUSN int64, changes []*syncv1.SyncChange) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to begin transaction in ApplyPushChanges", "error", err)
		return err
	}
	defer tx.Rollback()
	stmtCache := NewStmtCache(ctx, tx)
	defer stmtCache.Close()
	info := applyChangeInfo{
		userID:      userID,
		assignedUSN: assignedUSN,
	}

	for _, change := range changes {
		if err := s.applyChange(ctx, info, change, stmtCache); err != nil {
			return err
		}
	}
	if err := s.updateColSyncCursor(ctx, tx, info); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		s.logger.ErrorCtx(ctx, "failed to commit transaction in ApplyPushChanges", "error", err)
		return err
	}

	return nil
}

type applyChangeInfo struct {
	userID      string
	assignedUSN int64
}

func (s *PushChangeStore) updateColSyncCursor(ctx context.Context, tx *sql.Tx, info applyChangeInfo) error {
	result, err := tx.ExecContext(ctx, updateCollectionSyncCursorSQL, info.assignedUSN+1, info.userID)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to update collection sync cursor", "userID", info.userID, "syncCursorUSN", info.assignedUSN+1, "error", err)
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get rows affected after updating collection sync cursor", "userID", info.userID, "syncCursorUSN", info.assignedUSN+1, "error", err)
		return err
	}
	if rowsAffected == 0 {
		s.logger.ErrorCtx(ctx, "collection sync cursor update affected no rows", "userID", info.userID, "syncCursorUSN", info.assignedUSN+1)
		return errors.New("collection sync cursor update affected no rows")
	}

	return nil
}

func (s *PushChangeStore) applyChange(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	if change == nil {
		return fmt.Errorf("%w: nil sync change", errInvalidPushChange)
	}
	if change.GetUsn() != -1 {
		return fmt.Errorf("%w: usn must be -1: %d", errInvalidPushChange, change.GetUsn())
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

func (s *PushChangeStore) applyUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
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

func (s *PushChangeStore) applyDelete(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	entityID, err := uuidBytes(change.GetEntityId())
	if err != nil {
		s.logger.ErrorCtx(ctx, "invalid entity id in applyDelete", "entity_id", change.GetEntityId(), "error", err)
		return fmt.Errorf("%w: invalid entity id: %w", errInvalidPushChange, err)
	}

	switch change.GetEntityType() {
	case syncv1.EntityType_ENTITY_TYPE_REVIEW_LOG:
		s.logger.ErrorCtx(ctx, "review_log delete is not supported", "entity_id", change.GetEntityId())
		return fmt.Errorf("%w: review_log delete is not supported", errInvalidPushChange)
	case syncv1.EntityType_ENTITY_TYPE_CARD:
		return s.applySoftDelete(ctx, info, stmtCache, PushOpDeleteCard, entityID, change)
	case syncv1.EntityType_ENTITY_TYPE_NOTE:
		return s.applySoftDelete(ctx, info, stmtCache, PushOpDeleteNote, entityID, change)
	case syncv1.EntityType_ENTITY_TYPE_PROCESSING_NOTE:
		return s.applySoftDelete(ctx, info, stmtCache, PushOpDeleteProcessingNote, entityID, change)
	case syncv1.EntityType_ENTITY_TYPE_NOTE_TYPE:
		return s.applySoftDelete(ctx, info, stmtCache, PushOpDeleteNoteType, entityID, change)
	case syncv1.EntityType_ENTITY_TYPE_DECK:
		return s.applySoftDelete(ctx, info, stmtCache, PushOpDeleteDeck, entityID, change)
	case syncv1.EntityType_ENTITY_TYPE_COLLECTION:
		s.logger.ErrorCtx(ctx, "collection delete is not supported", "entity_id", change.GetEntityId())
		return fmt.Errorf("%w: collection delete is not supported", errInvalidPushChange)
	default:
		s.logger.ErrorCtx(ctx, "invalid entity type in applyDelete", "entity_type", change.GetEntityType())
		return fmt.Errorf("%w: invalid entity type", errInvalidPushChange)
	}
}

func (s *PushChangeStore) applyReviewLogUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetReviewLog()
	if payload == nil {
		return fmt.Errorf("%w: missing review_log payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		s.logger.ErrorCtx(ctx, "invalid review_log id", "id", entityID, "error", err)
		return fmt.Errorf("%w: invalid review_log id: %w", errInvalidPushChange, err)
	}
	cardID, err := uuidBytes(payload.CardId)
	if err != nil {
		s.logger.ErrorCtx(ctx, "invalid review_log card id", "card_id", payload.CardId, "error", err)
		return fmt.Errorf("%w: invalid review_log card id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertReviewLog)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get upsert review_log statement", "error", err)
		return err
	}

	_, err = stmt.ExecContext(ctx, info.userID, entityUUID,
		cardID, info.assignedUSN,
		payload.ReviewTime, payload.ScheduledDays,
		payload.Rating, payload.Difficulty, payload.Stability,
		payload.LearningSteps, payload.State, payload.Duration,
	)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to execute query in applyReviewLogChange", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.ReviewTime)
}

func (s *PushChangeStore) applyCardUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetCard()
	if payload == nil {
		return fmt.Errorf("%w: missing card payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		return fmt.Errorf("%w: invalid card id: %w", errInvalidPushChange, err)
	}
	noteID, err := uuidBytes(payload.NoteId)
	if err != nil {
		return fmt.Errorf("%w: invalid card note id: %w", errInvalidPushChange, err)
	}
	deckID, err := uuidBytes(payload.DeckId)
	if err != nil {
		return fmt.Errorf("%w: invalid card deck id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertCard)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityUUID, noteID, deckID, info.assignedUSN,
		payload.UpdatedAt, payload.Difficulty, payload.Stability, payload.ScheduledDays,
		payload.Due, nullableInt64(payload.LastReview), payload.Lapses, payload.LearningSteps,
		payload.Repetitions, payload.State, payload.Queue)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert card", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyNoteUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetNote()
	if payload == nil {
		return fmt.Errorf("%w: missing note payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		return fmt.Errorf("%w: invalid note id: %w", errInvalidPushChange, err)
	}
	noteTypeID, err := uuidBytes(payload.NoteTypeId)
	if err != nil {
		return fmt.Errorf("%w: invalid note note_type id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertNote)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityUUID, noteTypeID,
		info.assignedUSN,
		payload.CreatedAt, payload.UpdatedAt, nullableInt32(payload.SenseId), nullableString(payload.SortField),
		nullableString(payload.SearchFields), payload.FieldsJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert note", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyProcessingNoteUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetProcessingNote()
	if payload == nil {
		return fmt.Errorf("%w: missing processing_note payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		return fmt.Errorf("%w: invalid processing_note id: %w", errInvalidPushChange, err)
	}
	noteTypeID, err := uuidBytes(payload.NoteTypeId)
	if err != nil {
		return fmt.Errorf("%w: invalid processing_note note_type id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertProcessingNote)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityUUID, noteTypeID, info.assignedUSN,
		payload.CreatedAt, payload.UpdatedAt, nullableInt32(payload.SenseId), payload.FieldsJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert processing_note", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyNoteTypeUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetNoteType()
	if payload == nil {
		return fmt.Errorf("%w: missing note_type payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		return fmt.Errorf("%w: invalid note_type id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertNoteType)
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

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyDeckUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetDeck()
	if payload == nil {
		return fmt.Errorf("%w: missing deck payload", errInvalidPushChange)
	}

	entityID := change.GetEntityId()
	entityUUID, err := uuidBytes(entityID)
	if err != nil {
		return fmt.Errorf("%w: invalid deck id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertDeck)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityUUID,
		info.assignedUSN,
		payload.Name, payload.UpdatedAt,
		payload.NewCardsPerDay, payload.NewLearnedToday,
		payload.LearnedToday, payload.ReviewedToday, payload.ConfigJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert deck", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityUUID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applyCollectionUpsert(ctx context.Context, info applyChangeInfo, change *syncv1.SyncChange, stmtCache *StmtCache) error {
	payload := change.GetCollection()
	if payload == nil {
		return fmt.Errorf("%w: missing collection payload", errInvalidPushChange)
	}

	entityID, err := uuidBytes(change.GetEntityId())
	if err != nil {
		return fmt.Errorf("%w: invalid collection id: %w", errInvalidPushChange, err)
	}

	stmt, err := stmtCache.Get(ctx, PushOpUpsertCollection)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID,
		info.assignedUSN,
		payload.SqliteSchemaVersion, payload.CreatedAt, payload.UpdatedAt, payload.ConfigJson)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert collection", "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityID, change, payload.UpdatedAt)
}

func (s *PushChangeStore) applySoftDelete(ctx context.Context, info applyChangeInfo, stmtCache *StmtCache, op PushOp, entityID []byte, change *syncv1.SyncChange) error {
	if change.DeletedAt == nil {
		return fmt.Errorf("%w: delete missing deleted_at", errInvalidPushChange)
	}
	if change.GetPayload() != nil {
		return fmt.Errorf("%w: delete must not include payload", errInvalidPushChange)
	}

	stmt, err := stmtCache.Get(ctx, op)
	if err != nil {
		return err
	}
	deletedAt := change.GetDeletedAt()
	_, err = stmt.ExecContext(ctx, info.assignedUSN, deletedAt, info.userID, entityID)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to apply soft delete", "entity_id", change.GetEntityId(), "entity_type", change.GetEntityType(), "error", err)
		return err
	}

	return s.applySyncUnit(ctx, info, stmtCache, entityID, change, deletedAt)
}

func (s *PushChangeStore) applySyncUnit(ctx context.Context, info applyChangeInfo, stmtCache *StmtCache, entityID []byte, change *syncv1.SyncChange, updatedAt int64) error {
	entityType := change.GetEntityType()
	op := change.GetOp()

	stmt, err := stmtCache.Get(ctx, PushOpUpsertSyncUnit)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, info.userID, entityID, int32(entityType), int32(op), info.assignedUSN, updatedAt)
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to upsert sync_unit", "entity_type", entityType, "op", op, "error", err)
		return err
	}

	return nil
}
