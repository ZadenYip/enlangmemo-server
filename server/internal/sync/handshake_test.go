package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type fakeHandshakeStore struct {
	colInfo CollectionInfoForHandshake
	err     error
}

func (s *fakeHandshakeStore) GetColInfoForHandshake(ctx context.Context, userID string) (CollectionInfoForHandshake, error) {
	if s.err != nil {
		return CollectionInfoForHandshake{}, s.err
	}
	return s.colInfo, nil
}

type fakeSessionStore struct {
	mock.Mock

	result         CreateSessionResult
	err            error
	createdSession SyncSession
	createCalls    int
}

func newFakeSessionStore(t *testing.T, result CreateSessionResult) *fakeSessionStore {
	t.Helper()
	store := &fakeSessionStore{result: result}
	t.Cleanup(func() {
		store.AssertNotCalled(t, "GetSession", mock.Anything, mock.Anything)
		store.AssertNotCalled(t, "ClaimPushBatch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		store.AssertNotCalled(t, "MarkPushFinished", mock.Anything, mock.Anything, mock.Anything)
	})
	return store
}

func (s *fakeSessionStore) GetSession(ctx context.Context, userID string) (SyncSession, error) {
	args := s.Called(ctx, userID)
	return args.Get(0).(SyncSession), args.Error(1)
}

func (s *fakeSessionStore) CreateSession(ctx context.Context, session SyncSession) (CreateSessionResult, error) {
	s.createCalls++
	s.createdSession = session
	return s.result, s.err
}

func (s *fakeSessionStore) ClaimPushBatch(ctx context.Context, userID, sessionID string, currentBatchSeq int64) (ClaimPushBatchResult, error) {
	args := s.Called(ctx, userID, sessionID, currentBatchSeq)
	return args.Get(0).(ClaimPushBatchResult), args.Error(1)
}

func (s *fakeSessionStore) MarkPushFinished(ctx context.Context, userID, sessionID string) error {
	args := s.Called(ctx, userID, sessionID)
	return args.Error(0)
}

func TestHandshakeStatusAndSessionState(t *testing.T) {
	tests := []struct {
		name            string
		clientCursor    int64
		serverCursor    int64
		hasLocalChanges bool
		wantStatus      syncv1.HandshakeStatus
		wantState       SessionState
		wantBatchSeq    int64
		wantSyncCursor  int64
		wantCreate      bool
	}{
		{
			name:            "no remote changes with local changes",
			clientCursor:    10,
			serverCursor:    10,
			hasLocalChanges: true,
			wantStatus:      syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES,
			wantState:       SyncSessionStatePushing,
			wantBatchSeq:    1,
			wantSyncCursor:  10,
			// SessionID
			wantCreate: true,
		},
		{
			name:            "no remote changes without local changes",
			clientCursor:    10,
			serverCursor:    10,
			hasLocalChanges: false,
			wantStatus:      syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES,
			wantCreate:      false,
		},
		{
			name:           "need pull",
			clientCursor:   8,
			serverCursor:   10,
			wantStatus:     syncv1.HandshakeStatus_HANDSHAKE_STATUS_NEED_PULL,
			wantState:      SyncSessionStatePulling,
			wantBatchSeq:   1,
			wantSyncCursor: 8,
			wantCreate:     true,
		},
		{
			name:           "upload all",
			clientCursor:   12,
			serverCursor:   10,
			wantStatus:     syncv1.HandshakeStatus_HANDSHAKE_STATUS_UPLOAD_ALL,
			wantState:      SyncSessionStateAwaitingUploadAllConfirm,
			wantBatchSeq:   1,
			wantSyncCursor: 0,
			wantCreate:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), "userID", "user-1")
			sessionStore := newFakeSessionStore(t, CreateSessionCreated)
			handler := &SyncHandler{
				hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
					CollectionID:  "collection-1",
					SyncCursorUSN: tt.serverCursor,
				}},
				sessionStore: sessionStore,
			}

			resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
				CollectionId:        "collection-1",
				ClientNow:           time.Now().UnixMilli(),
				ClientSyncCursorUsn: tt.clientCursor,
				DeviceId:            "device-1",
				HasLocalChanges:     tt.hasLocalChanges,
			}))

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.Msg.Status)
			require.Equal(t, tt.serverCursor, resp.Msg.ServerSyncCursorUsn)

			var wantCreateInt int
			if tt.wantCreate {
				wantCreateInt = 1
			} else {
				wantCreateInt = 0
			}

			require.Equal(t, wantCreateInt, sessionStore.createCalls)
			if !tt.wantCreate {
				require.Nil(t, resp.Msg.SessionId)
				return
			}
			require.NotNil(t, resp.Msg.SessionId)
			require.NotEmpty(t, *resp.Msg.SessionId)
			require.Equal(t, "user-1", sessionStore.createdSession.UserID)
			require.Equal(t, "device-1", sessionStore.createdSession.DeviceID)
			require.Equal(t, tt.wantState, sessionStore.createdSession.State)
			require.Equal(t, tt.wantBatchSeq, sessionStore.createdSession.ExpectedBatchSeq)
			require.Equal(t, tt.wantSyncCursor, sessionStore.createdSession.SyncCursorUSN)
			require.Equal(t, tt.clientCursor, sessionStore.createdSession.CliSyncCursorUSNAtHandshake)
			require.Equal(t, tt.serverCursor, sessionStore.createdSession.SrvSyncCursorUSNAtHandshake)
			require.Equal(t, *resp.Msg.SessionId, sessionStore.createdSession.SessionID)
		})
	}
}

// TestHandshakeTimeSkewTooLargeDoesNotCreateSession 测试当客户端和服务器时间差超过 5 分钟时，握手返回 TIME_SKEW_TOO_LARGE
func TestHandshakeTimeSkewTooLargeDoesNotCreateSession(t *testing.T) {
	ctx := context.WithValue(context.Background(), "userID", "user-1")
	sessionStore := newFakeSessionStore(t, CreateSessionCreated)
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  "collection-1",
			SyncCursorUSN: 10,
		}},
		sessionStore: sessionStore,
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId: "collection-1",
		// 偏差 6 分钟，超过 5 分钟的阈值
		ClientNow:           time.Now().Add(6 * time.Minute).UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            "device-1",
	}))

	require.NoError(t, err)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_TIME_SKEW_TOO_LARGE, resp.Msg.Status)
	require.Equal(t, int64(10), resp.Msg.ServerSyncCursorUsn)
	require.Nil(t, resp.Msg.SessionId)
	require.Equal(t, 0, sessionStore.createCalls)
}

func TestHandshakeStoreErrors(t *testing.T) {
	// 测试 collection store 返回错误时，握手返回 INTERNAL 错误
	ctx := context.WithValue(context.Background(), "userID", "user-1")
	handler := &SyncHandler{
		hskStore:     &fakeHandshakeStore{err: errors.New("handshake store error")},
		sessionStore: newFakeSessionStore(t, CreateSessionCreated),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId: "collection-1",
		ClientNow:    time.Now().UnixMilli(),
	}))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))

}

// TestHandshakeSessionAlreadyExists 测试已经有 session 时，握手返回 LOCKED_BY_OTHER_CLIENT 状态
func TestHandshakeSessionAlreadyExists(t *testing.T) {
	// 测试 session store 返回已经存在会话时，握手返回 LOCKED_BY_OTHER_CLIENT 状态
	ctx := context.WithValue(context.Background(), "userID", "user-1")
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  "collection-1",
			SyncCursorUSN: 10,
		}},
		sessionStore: newFakeSessionStore(t, CreateSessionAlreadyExists),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId:        "collection-1",
		ClientNow:           time.Now().UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            "device-1",
		HasLocalChanges:     true,
	}))

	require.NoError(t, err)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_LOCKED_BY_OTHER_CLIENT, resp.Msg.Status)
}

func TestHandshakeCollectionIDMismatch(t *testing.T) {
	ctx := context.WithValue(context.Background(), "userID", "user-1")
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  "server-collection",
			SyncCursorUSN: 10,
		}},
		sessionStore: newFakeSessionStore(t, CreateSessionCreated),
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId:        "client-collection",
		ClientNow:           time.Now().UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            "device-1",
	}))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
