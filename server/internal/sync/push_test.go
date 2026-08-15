package sync

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type fakePushSessionStore struct {
	claimResult    ss.ClaimPushBatchLuaResult
	claimResultSet bool
	claimErr       error
	assignedUSN    int64
}

func (s *fakePushSessionStore) CreateSession(ctx context.Context, session ss.SyncSession) (ss.CreateSessionResult, error) {
	panic("CreateSession should not be called")
}

func (s *fakePushSessionStore) GetSession(ctx context.Context, userID string) (ss.SyncSession, error) {
	panic("GetSession should not be called")
}

func (s *fakePushSessionStore) ClaimPushBatch(ctx context.Context, userID, sessionID string, currentBatchSeq int64) (ss.ClaimPushBatchResult, error) {
	if !s.claimResultSet && s.claimErr == nil {
		return ss.ClaimPushBatchResult{LuaResult: ss.ClaimPushBatchLuaOK, AssignedUSN: s.assignedUSN}, nil
	}
	return ss.ClaimPushBatchResult{LuaResult: s.claimResult, AssignedUSN: s.assignedUSN}, s.claimErr
}

func (s *fakePushSessionStore) MarkPushFinished(ctx context.Context, userID, sessionID string) error {
	panic("MarkPushFinished should not be called")
}

func (s *fakePushSessionStore) FinishSync(ctx context.Context, userID, sessionID string, finishTime int64) error {
	panic("FinishSync should not be called")
}

func TestClaimPushBatch(t *testing.T) {
	const wantAssignedUSN int64 = 13

	tests := []struct {
		name        string
		claimResult ss.ClaimPushBatchLuaResult
		claimErr    error
		req         *syncv1.PushRequest
		wantCode    connect.Code
		wantError   bool
	}{
		{
			name:        "success",
			claimResult: ss.ClaimPushBatchLuaOK,
			req: &syncv1.PushRequest{
				SessionId: "session-1",
				BatchSeq:  2,
			},
		},
		{
			name:        "missing session",
			claimResult: ss.ClaimPushBatchLuaSessionNotFound,
			req:         &syncv1.PushRequest{SessionId: "session-1", BatchSeq: 1},
			wantCode:    connect.CodeFailedPrecondition,
			wantError:   true,
		},
		{
			name:        "session id mismatch",
			claimResult: ss.ClaimPushBatchLuaSessionIDMismatch,
			req:         &syncv1.PushRequest{SessionId: "other-session", BatchSeq: 1},
			wantCode:    connect.CodeFailedPrecondition,
			wantError:   true,
		},
		{
			name:        "batch seq mismatch",
			claimResult: ss.ClaimPushBatchLuaBatchSeqMismatch,
			req:         &syncv1.PushRequest{SessionId: "session-1", BatchSeq: 1},
			wantCode:    connect.CodeFailedPrecondition,
			wantError:   true,
		},
		{
			name:        "state mismatch",
			claimResult: ss.ClaimPushBatchLuaStateMismatch,
			req:         &syncv1.PushRequest{SessionId: "session-1", BatchSeq: 1},
			wantCode:    connect.CodeFailedPrecondition,
			wantError:   true,
		},
		{
			name:        "unknown lua result",
			claimResult: ss.ClaimPushBatchLuaResult(99),
			req:         &syncv1.PushRequest{SessionId: "session-1", BatchSeq: 1},
			wantCode:    connect.CodeInternal,
			wantError:   true,
		},
		{
			name:      "store error",
			claimErr:  errors.New("redis unavailable"),
			req:       &syncv1.PushRequest{SessionId: "session-1", BatchSeq: 1},
			wantCode:  connect.CodeInternal,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := "user-1"
			ctx := context.WithValue(context.Background(), "userID", userID)
			handler := &SyncHandler{
				sessionStore: &fakePushSessionStore{claimResult: tt.claimResult, claimResultSet: tt.claimResult != 0, claimErr: tt.claimErr, assignedUSN: wantAssignedUSN},
				logger:       logging.NewServerLog(),
			}

			result, err := handler.claimPushBatch(ctx, tt.req, userID)

			if tt.wantError {
				require.Equal(t, tt.wantCode, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, ss.ClaimPushBatchLuaOK, result.LuaResult)
			require.Equal(t, wantAssignedUSN, result.AssignedUSN)
		})
	}
}

func TestInvalidArgumentChange(t *testing.T) {
	deletedAt := int64(1_700_000_000_000)
	validDeckID := "018f3f3f-8f3f-7f3f-bf3f-8f3f8f3f8f3f"
	validDeckUpsert := func() *syncv1.SyncChange {
		return &syncv1.SyncChange{
			EntityId:   validDeckID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
			Op:         syncv1.ChangeOp_CHANGE_OP_UPSERT,
			Usn:        -1,
			Payload: &syncv1.SyncChange_Deck{Deck: &syncv1.DeckPayload{
				Name:            "deck",
				UpdatedAt:       1_700_000_000_000,
				NewCardsPerDay:  20,
				NewLearnedToday: 1,
				LearnedToday:    2,
				ReviewedToday:   3,
				ConfigJson:      `{}`,
			}},
		}
	}
	validDeckDelete := func() *syncv1.SyncChange {
		return &syncv1.SyncChange{
			EntityId:   validDeckID,
			EntityType: syncv1.EntityType_ENTITY_TYPE_DECK,
			Op:         syncv1.ChangeOp_CHANGE_OP_DELETE,
			DeletedAt:  &deletedAt,
			Usn:        -1,
		}
	}

	tests := []struct {
		name   string
		change *syncv1.SyncChange
	}{
		{
			name:   "nil change",
			change: nil,
		},
		{
			name: "usn must be -1",
			change: func() *syncv1.SyncChange {
				change := validDeckUpsert()
				change.Usn = 12
				return change
			}(),
		},
		{
			name: "unspecified entity type",
			change: func() *syncv1.SyncChange {
				change := validDeckUpsert()
				change.EntityType = syncv1.EntityType_ENTITY_TYPE_UNSPECIFIED
				return change
			}(),
		},
		{
			name: "entity_id must be valid uuid",
			change: func() *syncv1.SyncChange {
				change := validDeckDelete()
				change.EntityId = "not-a-uuid"
				return change
			}(),
		},
		{
			name: "delete change must include deleted_at",
			change: func() *syncv1.SyncChange {
				change := validDeckDelete()
				change.DeletedAt = nil
				return change
			}(),
		},
		{
			name: "delete change payload must be empty",
			change: func() *syncv1.SyncChange {
				change := validDeckDelete()
				change.Payload = validDeckUpsert().GetPayload()
				return change
			}(),
		},
		{
			name: "unspecified op",
			change: func() *syncv1.SyncChange {
				change := validDeckUpsert()
				change.Op = syncv1.ChangeOp_CHANGE_OP_UNSPECIFIED
				return change
			}(),
		},
	}

	store := &PushChangeStore{logger: logging.NewServerLog()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.applyChange(t.Context(), applyChangeInfo{}, tt.change, nil)
			require.ErrorIs(t, err, errInvalidPushChange)
		})
	}
}
