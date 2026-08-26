package sync

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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
	changeCount    int
}

func (s *fakePushSessionStore) CreateSession(ctx context.Context, session ss.SyncSession) (ss.CreateSessionResult, error) {
	panic("CreateSession should not be called")
}

func (s *fakePushSessionStore) GetSession(ctx context.Context, userID int64) (ss.SyncSession, error) {
	panic("GetSession should not be called")
}

func (s *fakePushSessionStore) ClaimPushBatch(ctx context.Context, userID int64, sessionID string, currentBatchSeq int32, changeCount int) (ss.ClaimPushBatchResult, error) {
	s.changeCount = changeCount
	if !s.claimResultSet && s.claimErr == nil {
		return ss.ClaimPushBatchResult{LuaResult: ss.ClaimPushBatchLuaOK, AssignedStartUSN: s.assignedUSN}, nil
	}
	return ss.ClaimPushBatchResult{LuaResult: s.claimResult, AssignedStartUSN: s.assignedUSN}, s.claimErr
}

func (s *fakePushSessionStore) MarkPushFinished(ctx context.Context, userID int64, sessionID string, curBatchSeq int32) error {
	panic("MarkPushFinished should not be called")
}

func (s *fakePushSessionStore) ClaimPullBatch(ctx context.Context, userID int64, sessionID string, currentBatchSeq int32) (ss.ClaimPullBatchResult, error) {
	panic("ClaimPullBatch should not be called")
}

func (s *fakePushSessionStore) UpdatePullProgress(ctx context.Context, userID int64, sessionID string, remainingPullEntityQueue string, syncCursorUSN int64) error {
	panic("UpdatePullProgress should not be called")
}

func (s *fakePushSessionStore) MarkPullFinished(ctx context.Context, userID int64, sessionID string) error {
	panic("MarkPullFinished should not be called")
}

func (s *fakePushSessionStore) FinishSync(ctx context.Context, userID int64, sessionID string, finishTime int64) error {
	panic("FinishSync should not be called")
}

func TestClaimPushBatch(t *testing.T) {
	const wantAssignedStartUSN int64 = 13

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
				Changes: []*syncv1.SyncChange{
					{Usn: -1},
					{Usn: -1},
				},
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
			userID := int64(10001)
			ctx := context.WithValue(context.Background(), "userID", userID)
			sessionStore := &fakePushSessionStore{claimResult: tt.claimResult, claimResultSet: tt.claimResult != 0, claimErr: tt.claimErr, assignedUSN: wantAssignedStartUSN}
			handler := &SyncHandler{
				sessionStore: sessionStore,
				logger:       logging.NewServerLog(),
			}

			result, err := handler.claimPushBatch(ctx, tt.req, userID)

			if tt.wantError {
				require.Equal(t, tt.wantCode, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, ss.ClaimPushBatchLuaOK, result.LuaResult)
			require.Equal(t, wantAssignedStartUSN, result.AssignedStartUSN)
			require.Equal(t, len(tt.req.GetChanges()), sessionStore.changeCount)
		})
	}
}

func TestInvalidArgumentChange(t *testing.T) {
	deletedAt := int64(1_700_000_000_000)
	validDeckUUID := uuid.Must(uuid.NewV7())
	validDeckID := validDeckUUID[:]
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
