package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type fakeCollectionStore struct {
	colInfo ColInfoForHandshake
	err     error
}

func (s *fakeCollectionStore) GetServerSyncCursorUSN(ctx context.Context, userID string, colID string) (int64, error) {
	return 0, nil
}

func (s *fakeCollectionStore) GetColInfoForHandshake(ctx context.Context, userID string) (ColInfoForHandshake, error) {
	if s.err != nil {
		return ColInfoForHandshake{}, s.err
	}
	return s.colInfo, nil
}

type fakeSessionStore struct {
	result         CreateSessionResult
	err            error
	createdSession SyncSession
	createCalls    int
}

func (s *fakeSessionStore) CreateSession(ctx context.Context, session SyncSession) (CreateSessionResult, error) {
	s.createCalls++
	s.createdSession = session
	return s.result, s.err
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
			sessionStore := &fakeSessionStore{result: CreateSessionCreated}
			handler := &SyncHandler{
				colStore: &fakeCollectionStore{colInfo: ColInfoForHandshake{
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
	sessionStore := &fakeSessionStore{result: CreateSessionCreated}
	handler := &SyncHandler{
		colStore: &fakeCollectionStore{colInfo: ColInfoForHandshake{
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
		colStore:     &fakeCollectionStore{err: errors.New("collection store error")},
		sessionStore: &fakeSessionStore{result: CreateSessionCreated},
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
		colStore: &fakeCollectionStore{colInfo: ColInfoForHandshake{
			CollectionID:  "collection-1",
			SyncCursorUSN: 10,
		}},
		sessionStore: &fakeSessionStore{result: CreateSessionAlreadyExists},
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
		colStore: &fakeCollectionStore{colInfo: ColInfoForHandshake{
			CollectionID:  "server-collection",
			SyncCursorUSN: 10,
		}},
		sessionStore: &fakeSessionStore{result: CreateSessionCreated},
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
