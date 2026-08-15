package interceptor

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

type fakeOAStoreForAuthInterceptor struct {
	userID   string
	err      error
	gotToken string
	getCalls int
}

func (s *fakeOAStoreForAuthInterceptor) GetClientInfo(ctx context.Context, clientID string) (oauth.OAClientInfo, error) {
	return oauth.OAClientInfo{}, nil
}

func (s *fakeOAStoreForAuthInterceptor) GenCodeStoreSession(ctx context.Context, authoInfo oauth.AuthorizationInfo) (string, error) {
	return "", nil
}

func (s *fakeOAStoreForAuthInterceptor) ConsumeCodeSession(ctx context.Context, authCode string) (oauth.OAuthSession, error) {
	return oauth.OAuthSession{}, nil
}

func (s *fakeOAStoreForAuthInterceptor) GenAccessToken(ctx context.Context, clientID, userID string) (string, error) {
	return "", nil
}

// GetUserIDByAccessToken 用于模拟获取用户 ID 的方法，会保存从 header 获取的 token
func (s *fakeOAStoreForAuthInterceptor) GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error) {
	s.getCalls++
	s.gotToken = accessToken
	return s.userID, s.err
}

func (s *fakeOAStoreForAuthInterceptor) GetTokenInfoByAccessToken(ctx context.Context, accessToken string) (oauth.TokenInfo, error) {
	return oauth.TokenInfo{}, nil
}

func (s *fakeOAStoreForAuthInterceptor) RevokeAccessToken(ctx context.Context, accessToken, clientID string) error {
	return nil
}

func TestAuthInterceptorSuccess(t *testing.T) {
	store := &fakeOAStoreForAuthInterceptor{userID: "user-1"}
	req := connect.NewRequest(&syncv1.HandshakeRequest{})
	req.Header().Set("Authorization", "Bearer access-token")

	var nextCalled bool
	next := func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		// 验证 userID 被中间件是否加进了 context 中
		require.Equal(t, "user-1", ctx.Value("userID"))
		require.Same(t, req, r)
		return connect.NewResponse(&syncv1.HandshakeResponse{}), nil
	}

	resp, err := NewAuthInterceptor(store)(next)(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, nextCalled)
	require.Equal(t, 1, store.getCalls)

	// 这里会验证从 header 中获取的 token 是否正确传给了 store
	require.Equal(t, "access-token", store.gotToken)
}

func TestAuthInterceptorRejectsMissingToken(t *testing.T) {
	store := &fakeOAStoreForAuthInterceptor{userID: "user-1"}
	req := connect.NewRequest(&syncv1.HandshakeRequest{})

	resp, err := NewAuthInterceptor(store)(mustNotCallNext(t))(context.Background(), req)

	require.Nil(t, resp)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, 0, store.getCalls)
}

func TestAuthInterceptorRejectsInvalidToken(t *testing.T) {
	store := &fakeOAStoreForAuthInterceptor{err: oauth.ErrAccessTokenNotFound}
	req := connect.NewRequest(&syncv1.HandshakeRequest{})
	req.Header().Set("Authorization", "Bearer invalid-token")

	resp, err := NewAuthInterceptor(store)(mustNotCallNext(t))(context.Background(), req)

	require.Nil(t, resp)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, 1, store.getCalls)
	require.Equal(t, "invalid-token", store.gotToken)
}

func TestAuthInterceptorReturnsInternalOnStoreError(t *testing.T) {
	store := &fakeOAStoreForAuthInterceptor{err: errors.New("store error")}
	req := connect.NewRequest(&syncv1.HandshakeRequest{})
	req.Header().Set("Authorization", "Bearer access-token")

	resp, err := NewAuthInterceptor(store)(mustNotCallNext(t))(context.Background(), req)

	require.Nil(t, resp)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	require.Equal(t, 1, store.getCalls)
	require.Equal(t, "access-token", store.gotToken)
}

func mustNotCallNext(t *testing.T) connect.UnaryFunc {
	t.Helper()
	return func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("next should not be called")
		return nil, nil
	}
}
