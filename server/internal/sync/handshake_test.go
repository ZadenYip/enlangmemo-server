package sync

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type fakeHandshakeStore struct {
	colInfo         CollectionInfoForHandshake
	pullEntityQueue string
	err             error
}

func (s *fakeHandshakeStore) GetColInfoForHandshake(ctx context.Context, userID int64) (CollectionInfoForHandshake, error) {
	if s.err != nil {
		return CollectionInfoForHandshake{}, s.err
	}
	return s.colInfo, nil
}

func (s *fakeHandshakeStore) GetPullEntityQueueForHandshake(ctx context.Context, userID int64, minUSNInclusive, maxUSNExclusive int64) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.pullEntityQueue, nil
}

type fakeSessionStore struct {
	mock.Mock

	result         ss.CreateSessionResult
	err            error
	createdSession ss.SyncSession
	createCalls    int
}

func newFakeSessionStore(t *testing.T, result ss.CreateSessionResult) *fakeSessionStore {
	t.Helper()
	store := &fakeSessionStore{result: result}
	t.Cleanup(func() {
		store.AssertNotCalled(t, "GetSession", mock.Anything, mock.Anything)
		store.AssertNotCalled(t, "ClaimPushBatch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		store.AssertNotCalled(t, "MarkPushFinished", mock.Anything, mock.Anything, mock.Anything)
		store.AssertNotCalled(t, "FinishSync", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
	return store
}

func (s *fakeSessionStore) GetSession(ctx context.Context, userID int64) (ss.SyncSession, error) {
	args := s.Called(ctx, userID)
	return args.Get(0).(ss.SyncSession), args.Error(1)
}

func (s *fakeSessionStore) CreateSession(ctx context.Context, session ss.SyncSession) (ss.CreateSessionResult, error) {
	s.createCalls++
	s.createdSession = session
	return s.result, s.err
}

func (s *fakeSessionStore) ClaimPushBatch(ctx context.Context, userID int64, sessionID string, currentBatchSeq int32, changeCount int) (ss.ClaimPushBatchResult, error) {
	args := s.Called(ctx, userID, sessionID, currentBatchSeq, changeCount)
	return args.Get(0).(ss.ClaimPushBatchResult), args.Error(1)
}

func (s *fakeSessionStore) MarkPushFinished(ctx context.Context, userID int64, sessionID string) error {
	args := s.Called(ctx, userID, sessionID)
	return args.Error(0)
}

func (s *fakeSessionStore) ClaimPullBatch(ctx context.Context, userID int64, sessionID string, currentBatchSeq int32) (ss.ClaimPullBatchResult, error) {
	args := s.Called(ctx, userID, sessionID, currentBatchSeq)
	return args.Get(0).(ss.ClaimPullBatchResult), args.Error(1)
}

func (s *fakeSessionStore) UpdatePullProgress(ctx context.Context, userID int64, sessionID string, remainingPullEntityQueue string, syncCursorUSN int64) error {
	args := s.Called(ctx, userID, sessionID, remainingPullEntityQueue, syncCursorUSN)
	return args.Error(0)
}

func (s *fakeSessionStore) MarkPullFinished(ctx context.Context, userID int64, sessionID string) error {
	args := s.Called(ctx, userID, sessionID)
	return args.Error(0)
}

func (s *fakeSessionStore) FinishSync(ctx context.Context, userID int64, sessionID string, finishTime int64) error {
	args := s.Called(ctx, userID, sessionID, finishTime)
	return args.Error(0)
}

func TestHandshakeStatusAndSessionState(t *testing.T) {
	tests := []struct {
		name            string
		clientCursor    int64
		serverCursor    int64
		serverLastSync  int64
		hasLocalChanges bool
		wantStatus      syncv1.HandshakeStatus
		wantState       ss.SessionState
		wantBatchSeq    int64
		wantSyncCursor  int64
		wantEntityQueue string
		wantCreate      bool
	}{
		{
			name:            "no remote changes with local changes",
			clientCursor:    10,
			serverCursor:    10,
			serverLastSync:  1_800_000_000_000,
			hasLocalChanges: true,
			wantStatus:      syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES,
			wantState:       ss.SyncSessionStatePushing,
			wantBatchSeq:    1,
			wantSyncCursor:  10,
			// SessionID
			wantCreate: true,
		},
		{
			name:            "no remote changes without local changes",
			clientCursor:    10,
			serverCursor:    10,
			serverLastSync:  1_800_000_000_100,
			hasLocalChanges: false,
			wantStatus:      syncv1.HandshakeStatus_HANDSHAKE_STATUS_NO_REMOTE_CHANGES,
			wantCreate:      false,
		},
		{
			name:            "need pull",
			clientCursor:    8,
			serverCursor:    10,
			serverLastSync:  1_800_000_000_200,
			wantStatus:      syncv1.HandshakeStatus_HANDSHAKE_STATUS_NEED_PULL,
			wantState:       ss.SyncSessionStatePulling,
			wantBatchSeq:    1,
			wantSyncCursor:  8,
			wantEntityQueue: "1",
			wantCreate:      true,
		},
		{
			name:           "upload all",
			clientCursor:   12,
			serverCursor:   10,
			serverLastSync: 1_800_000_000_300,
			wantStatus:     syncv1.HandshakeStatus_HANDSHAKE_STATUS_UPLOAD_ALL,
			wantState:      ss.SyncSessionStateAwaitingUploadAllConfirm,
			wantBatchSeq:   1,
			wantSyncCursor: 0,
			wantCreate:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), "userID", int64(10001))
			deviceID := []byte("0123456789abcdef")
			mockStore := newFakeSessionStore(t, ss.CreateSessionCreated)
			handler := &SyncHandler{
				hskStore: &fakeHandshakeStore{pullEntityQueue: tt.wantEntityQueue, colInfo: CollectionInfoForHandshake{
					CollectionID:  []byte("collection-1"),
					SyncCursorUSN: tt.serverCursor,
					LastSyncTime:  tt.serverLastSync,
				}},
				sessionStore: mockStore,
				logger:       logging.NewServerLog(),
			}

			resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
				CollectionId:        []byte("collection-1"),
				ClientNow:           time.Now().UnixMilli(),
				ClientSyncCursorUsn: tt.clientCursor,
				DeviceId:            deviceID,
				HasLocalChanges:     tt.hasLocalChanges,
			}))

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.Msg.Status)
			require.Equal(t, tt.serverCursor, resp.Msg.ServerSyncCursorUsn)
			require.Equal(t, tt.serverLastSync, resp.Msg.ServerLastSyncTime)

			var wantCreateInt int
			if tt.wantCreate {
				wantCreateInt = 1
			} else {
				wantCreateInt = 0
			}

			require.Equal(t, wantCreateInt, mockStore.createCalls)
			if !tt.wantCreate {
				require.Nil(t, resp.Msg.SessionId)
				return
			}
			require.NotNil(t, resp.Msg.SessionId)
			require.NotEmpty(t, *resp.Msg.SessionId)
			require.Equal(t, int64(10001), mockStore.createdSession.UserID)
			require.Equal(t, hex.EncodeToString(deviceID), mockStore.createdSession.DeviceID)
			require.Equal(t, tt.wantState, mockStore.createdSession.State)
			require.Equal(t, tt.wantBatchSeq, mockStore.createdSession.ExpectedBatchSeq)
			require.Equal(t, tt.wantSyncCursor, mockStore.createdSession.SyncCursorUSN)
			require.Equal(t, tt.wantEntityQueue, mockStore.createdSession.PullEntityQueue)
			require.Equal(t, tt.clientCursor, mockStore.createdSession.CliSyncCursorUSNAtHandshake)
			require.Equal(t, tt.serverCursor, mockStore.createdSession.SrvSyncCursorUSNAtHandshake)
			require.Equal(t, *resp.Msg.SessionId, mockStore.createdSession.SessionID)
		})
	}
}

// TestHandshakeTimeSkewTooLargeDoesNotCreateSession 测试当客户端和服务器时间差超过 5 分钟时，握手返回 TIME_SKEW_TOO_LARGE
func TestHandshakeTimeSkewTooLargeDoesNotCreateSession(t *testing.T) {
	ctx := context.WithValue(context.Background(), "userID", int64(10001))
	sessionStore := newFakeSessionStore(t, ss.CreateSessionCreated)
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  []byte("collection-1"),
			SyncCursorUSN: 10,
		}},
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId: []byte("collection-1"),
		// 偏差 6 分钟，超过 5 分钟的阈值
		ClientNow:           time.Now().Add(6 * time.Minute).UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            []byte("device-1"),
	}))

	require.NoError(t, err)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_TIME_SKEW_TOO_LARGE, resp.Msg.Status)
	require.Equal(t, int64(10), resp.Msg.ServerSyncCursorUsn)
	require.Nil(t, resp.Msg.SessionId)
	require.Equal(t, 0, sessionStore.createCalls)
}

func TestHandshakeStoreErrors(t *testing.T) {
	// 测试 collection store 返回错误时，握手返回 INTERNAL 错误
	ctx := context.WithValue(context.Background(), "userID", int64(10001))
	handler := &SyncHandler{
		hskStore:     &fakeHandshakeStore{err: errors.New("handshake store error")},
		sessionStore: newFakeSessionStore(t, ss.CreateSessionCreated),
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId: []byte("collection-1"),
		ClientNow:    time.Now().UnixMilli(),
	}))

	require.Nil(t, resp)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))

}

// TestHandshakeSessionAlreadyExists 测试已经有 session 时，握手返回 LOCKED_BY_OTHER_CLIENT 状态
func TestHandshakeSessionAlreadyExists(t *testing.T) {
	// 测试 session store 返回已经存在会话时，握手返回 LOCKED_BY_OTHER_CLIENT 状态
	ctx := context.WithValue(context.Background(), "userID", int64(10001))
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  []byte("collection-1"),
			SyncCursorUSN: 10,
		}},
		sessionStore: newFakeSessionStore(t, ss.CreateSessionAlreadyExists),
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId:        []byte("collection-1"),
		ClientNow:           time.Now().UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            []byte("device-1"),
		HasLocalChanges:     true,
	}))

	require.NoError(t, err)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_LOCKED_BY_OTHER_CLIENT, resp.Msg.Status)
}

func TestHandshakeCollectionIDMismatch(t *testing.T) {
	ctx := context.WithValue(context.Background(), "userID", int64(10001))
	sessionStore := newFakeSessionStore(t, ss.CreateSessionCreated)
	handler := &SyncHandler{
		hskStore: &fakeHandshakeStore{colInfo: CollectionInfoForHandshake{
			CollectionID:  []byte("server-collection"),
			SyncCursorUSN: 10,
		}},
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}

	resp, err := handler.Handshake(ctx, connect.NewRequest(&syncv1.HandshakeRequest{
		CollectionId:        []byte("client-collection"),
		ClientNow:           time.Now().UnixMilli(),
		ClientSyncCursorUsn: 10,
		DeviceId:            []byte("device-1"),
	}))

	require.NoError(t, err)
	require.Equal(t, syncv1.HandshakeStatus_HANDSHAKE_STATUS_COLLECTION_ID_MISMATCH, resp.Msg.Status)
	require.Equal(t, int64(10), resp.Msg.ServerSyncCursorUsn)
	require.Equal(t, []byte("server-collection"), resp.Msg.GetCollectionId())
	require.Nil(t, resp.Msg.SessionId)
	require.Zero(t, sessionStore.createCalls)
}
