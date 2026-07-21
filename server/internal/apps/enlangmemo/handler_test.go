package enlangmemo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
)

const testUserID = "user-id"

type mockAccessTokenStore struct {
	mock.Mock
}

type mockUserProfileStore struct {
	mock.Mock
}

func (s *mockAccessTokenStore) GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error) {
	args := s.Called(ctx, accessToken)
	return args.String(0), args.Error(1)
}

func (s *mockUserProfileStore) GetUserProfile(ctx context.Context, userID string) (auth.UserProfile, error) {
	args := s.Called(ctx, userID)
	return args.Get(0).(auth.UserProfile), args.Error(1)
}

func newMeRequest(auth string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/v1/apps/enlangmemo/me", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func newTestHandler(tokenStore AccessTokenStore, users UserProfileStore) *Handler {
	return NewHandler(tokenStore, users, logging.NewServerLog())
}

// TestMeReturnsUserProfile 测试正常能否返回当前登录用户信息
func TestMeReturnsUserProfile(t *testing.T) {
	tokenStore := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)
	tokenStore.On("GetUserIDByAccessToken", mock.Anything, "access-token").
		Return(testUserID, nil).
		Once()
	users.On("GetUserProfile", mock.Anything, testUserID).
		Return(auth.UserProfile{
			UserID:   testUserID,
			LoginID:  "alice",
			Nickname: "Alice",
		}, nil).
		Once()

	rr := httptest.NewRecorder()
	newTestHandler(tokenStore, users).me(rr, newMeRequest("Bearer access-token"))

	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	require.JSONEq(t, `{
		"user_id": "user-id",
		"login_id": "alice",
		"nickname": "Alice"
	}`, rr.Body.String())
	tokenStore.AssertExpectations(t)
	users.AssertExpectations(t)
}

// TestMeRejectsMissingAuthorization 测试缺少 Authorization 头时是否返回 401 Unauthorized
func TestMeRejectsMissingAuthorization(t *testing.T) {
	store := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)

	rr := httptest.NewRecorder()
	newTestHandler(store, users).me(rr, newMeRequest(""))

	require.Equal(t, http.StatusUnauthorized, rr.Code, "body = %s", rr.Body.String())
	require.JSONEq(t, `{
		"error": {
			"code": 401,
			"message": "Invalid or missing access token",
			"status": "UNAUTHENTICATED",
			"details": []
		}
	}`, rr.Body.String())

	// 没有 header 会被直接拒绝，不会调用相关的 store
	store.AssertNotCalled(t, "GetUserIDByAccessToken", mock.Anything, mock.Anything)
	users.AssertNotCalled(t, "GetUserProfile", mock.Anything, mock.Anything)
}

// TestMeRejectsUnsupportedAuthorizationScheme 测试不支持的 Authorization scheme 时是否返回 401 Unauthorized
func TestMeRejectsUnsupportedAuthorizationScheme(t *testing.T) {
	store := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)

	rr := httptest.NewRecorder()
	// 这里传的是 Basic，而不是 Bearer 所以会拒绝
	newTestHandler(store, users).me(rr, newMeRequest("Basic access-token"))

	require.Equal(t, http.StatusUnauthorized, rr.Code, "body = %s", rr.Body.String())
	store.AssertNotCalled(t, "GetUserIDByAccessToken", mock.Anything, mock.Anything)
	users.AssertNotCalled(t, "GetUserProfile", mock.Anything, mock.Anything)
}

// TestMeRejectsMissingAccessToken 测试缺少 access token 时是否返回 401 Unauthorized
func TestMeRejectsUnknownAccessToken(t *testing.T) {
	store := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)
	store.On("GetUserIDByAccessToken", mock.Anything, "missing-token").
		Return("", oauth.ErrAccessTokenNotFound).
		Once()

	rr := httptest.NewRecorder()
	newTestHandler(store, users).me(rr, newMeRequest("Bearer missing-token"))

	require.Equal(t, http.StatusUnauthorized, rr.Code, "body = %s", rr.Body.String())
	store.AssertExpectations(t)
	users.AssertNotCalled(t, "GetUserProfile", mock.Anything, mock.Anything)
}

// TestMeStoreErrorReturnsInternal 测试当 token store 返回错误时，是否返回 500 Internal Server Error
func TestMeStoreErrorReturnsInternal(t *testing.T) {
	store := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)
	store.On("GetUserIDByAccessToken", mock.Anything, "access-token").
		Return("", errors.New("redis failed")).
		Once()

	rr := httptest.NewRecorder()
	newTestHandler(store, users).me(rr, newMeRequest("Bearer access-token"))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	require.JSONEq(t, `{
		"error": {
			"code": 500,
			"details": [],
			"message": "Internal server error",
			"status": "INTERNAL"
		}
	}`, rr.Body.String())
	store.AssertExpectations(t)
	users.AssertNotCalled(t, "GetUserProfile", mock.Anything, mock.Anything)
}

// TestMeMissingUserProfileReturnsInternal 测试 token 有效但用户不存在时是否返回 500 Internal Server Error
func TestMeMissingUserProfileReturnsInternal(t *testing.T) {
	tokenStore := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)
	tokenStore.On("GetUserIDByAccessToken", mock.Anything, "access-token").
		Return(testUserID, nil).
		Once()

	// 模拟用户不存在的情况，返回 auth.ErrUserNotFound
	users.On("GetUserProfile", mock.Anything, testUserID).
		Return(auth.UserProfile{}, auth.ErrUserNotFound).
		Once()

	rr := httptest.NewRecorder()
	newTestHandler(tokenStore, users).me(rr, newMeRequest("Bearer access-token"))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	tokenStore.AssertExpectations(t)
	users.AssertExpectations(t)
}

// TestMeUserProfileStoreErrorReturnsInternal 测试查询用户信息失败时是否返回 500 Internal Server Error
func TestMeUserProfileStoreErrorReturnsInternal(t *testing.T) {
	tokenStore := new(mockAccessTokenStore)
	users := new(mockUserProfileStore)
	tokenStore.On("GetUserIDByAccessToken", mock.Anything, "access-token").
		Return(testUserID, nil).
		Once()
	// 数据库查询失败
	users.On("GetUserProfile", mock.Anything, testUserID).
		Return(auth.UserProfile{}, errors.New("database failed")).
		Once()

	rr := httptest.NewRecorder()
	newTestHandler(tokenStore, users).me(rr, newMeRequest("Bearer access-token"))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	tokenStore.AssertExpectations(t)
	users.AssertExpectations(t)
}
