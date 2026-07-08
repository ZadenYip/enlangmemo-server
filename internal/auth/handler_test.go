package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockUserStore struct {
	mock.Mock
}

func (store *mockUserStore) CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (string, error) {
	args := store.Called(ctx, loginID, nickname, passwordHash)
	return args.String(0), args.Error(1)
}

func (store *mockUserStore) GetPasswordHash(ctx context.Context, loginID string) (string, string, error) {
	args := store.Called(ctx, loginID)
	return args.String(0), args.String(1), args.Error(2)
}

func passwordHashMatcher(password string) any {
	return mock.MatchedBy(func(passwordHash string) bool {
		match, err := argon2id.ComparePasswordAndHash(password, passwordHash)
		return err == nil && match
	})
}

func newRegisterRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// 测试名字长度超过最大限制的情况
func TestRegisterNameTooLong(t *testing.T) {
	userStore := new(mockUserStore)
	handler := newTestHandler(userStore, new(mockSSOStore))

	rr := httptest.NewRecorder()
	handler.register(rr, newRegisterRequest(`{"loginId":"abcdefghijklmnopq","nickname":"Alice","password":"password"}`))

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	userStore.AssertExpectations(t)
}

// 测试用户已经存在的情况
func TestRegisterUserAlreadyExists(t *testing.T) {
	userStore := new(mockUserStore)
	userStore.On("CreateUser", mock.Anything, "alice", "Alice", passwordHashMatcher("password")).
		Return("", ErrUserAlreadyExists)
	handler := newTestHandler(userStore, new(mockSSOStore))

	rr := httptest.NewRecorder()
	handler.register(rr, newRegisterRequest(`{"loginId":"alice","nickname":"Alice","password":"password"}`))

	require.Equal(t, http.StatusConflict, rr.Code, "body = %s", rr.Body.String())
	userStore.AssertExpectations(t)
}

// 测试如果内部 userStore 报错是否返回 500 错误（StatusInternalServerError）
func TestRegisterStoreError(t *testing.T) {
	userStore := new(mockUserStore)
	userStore.On("CreateUser", mock.Anything, "alice", "Alice", passwordHashMatcher("password")).
		Return("", errors.New("store error"))
	handler := newTestHandler(userStore, new(mockSSOStore))

	rr := httptest.NewRecorder()
	handler.register(rr, newRegisterRequest(`{"loginId":"alice","nickname":"Alice","password":"password"}`))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	userStore.AssertExpectations(t)
}

// 测试正常注册
func TestRegisterSuccess(t *testing.T) {
	userStore := new(mockUserStore)
	userStore.On("CreateUser", mock.Anything, "alice", "Alice", passwordHashMatcher("password")).
		Return("user-id", nil)
	handler := newTestHandler(userStore, new(mockSSOStore))

	rr := httptest.NewRecorder()
	handler.register(rr, newRegisterRequest(`{"loginId":"alice","nickname":"Alice","password":"password"}`))

	require.Equal(t, http.StatusCreated, rr.Code, "body = %s", rr.Body.String())
	userStore.AssertExpectations(t)
}

type mockSSOStore struct {
	mock.Mock
}

func (store *mockSSOStore) Create(ctx context.Context, userID string) (string, error) {
	args := store.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (store *mockSSOStore) GetUserID(ctx context.Context, sessionID string) (string, error) {
	args := store.Called(ctx, sessionID)
	return args.String(0), args.Error(1)
}

func (store *mockSSOStore) Delete(ctx context.Context, sessionID string) error {
	args := store.Called(ctx, sessionID)
	return args.Error(0)
}

func (store *mockSSOStore) Logout(ctx context.Context, sessionID string) error {
	args := store.Called(ctx, sessionID)
	return args.Error(0)
}

func newLoginRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newLogoutRequest(sessionID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	if sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "__Host-sso_token",
			Value: sessionID,
		})
	}
	return req
}

func newTestHandler(userStore *mockUserStore, ssoStore *mockSSOStore) *AuthHandler {
	return NewAuthHandler(userStore, ssoStore, discardLogger{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

type discardLogger struct {
	logger *slog.Logger
}

func (l discardLogger) Info() *slog.Logger {
	return l.logger
}

func (l discardLogger) Error() *slog.Logger {
	return l.logger
}

func (l discardLogger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l discardLogger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l discardLogger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

// 测试登录用户不存在的情况
func TestLoginUserNotFound(t *testing.T) {
	userStore := new(mockUserStore)
	userStore.On("GetPasswordHash", mock.Anything, "alice").
		Return("", "", ErrUserNotFound)
	ssoStore := new(mockSSOStore)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.login(rr, newLoginRequest(`{"loginId":"alice","password":"password"}`))

	require.Equal(t, http.StatusNotFound, rr.Code, "body = %s", rr.Body.String())
	require.Empty(t, rr.Result().Cookies())
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试退出登录时没有 session cookie 的情况
func TestLogoutMissingCookie(t *testing.T) {
	userStore := new(mockUserStore)
	ssoStore := new(mockSSOStore)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.logout(rr, newLogoutRequest(""))

	cookies := rr.Result().Cookies()
	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	require.Len(t, cookies, 1)
	require.Equal(t, "__Host-sso_token", cookies[0].Name)
	require.Equal(t, "", cookies[0].Value)
	require.Equal(t, -1, cookies[0].MaxAge)
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试退出登录时 session cookie 为空的情况
func TestLogoutEmptyCookie(t *testing.T) {
	userStore := new(mockUserStore)
	ssoStore := new(mockSSOStore)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-sso_token", Value: ""})
	handler.logout(rr, req)

	cookies := rr.Result().Cookies()
	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	require.Len(t, cookies, 1)
	require.Equal(t, "__Host-sso_token", cookies[0].Name)
	require.Equal(t, "", cookies[0].Value)
	require.Equal(t, -1, cookies[0].MaxAge)
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试退出登录时 session store 报错的情况
func TestLogoutStoreError(t *testing.T) {
	userStore := new(mockUserStore)
	ssoStore := new(mockSSOStore)
	ssoStore.On("Logout", mock.Anything, "session-id").
		Return(errors.New("store error"))
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.logout(rr, newLogoutRequest("session-id"))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试退出登录成功的情况
func TestLogoutSuccess(t *testing.T) {
	userStore := new(mockUserStore)
	ssoStore := new(mockSSOStore)
	ssoStore.On("Logout", mock.Anything, "session-id").
		Return(nil)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.logout(rr, newLogoutRequest("session-id"))

	cookies := rr.Result().Cookies()
	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	require.Len(t, cookies, 1)
	require.Equal(t, "__Host-sso_token", cookies[0].Name)
	require.Equal(t, "", cookies[0].Value)
	require.Equal(t, -1, cookies[0].MaxAge)
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试登录时 userStore 报错的情况
func TestLoginStoreError(t *testing.T) {
	userStore := new(mockUserStore)
	userStore.On("GetPasswordHash", mock.Anything, "alice").
		Return("", "", errors.New("store error"))
	ssoStore := new(mockSSOStore)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.login(rr, newLoginRequest(`{"loginId":"alice","password":"password"}`))

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
	require.Empty(t, rr.Result().Cookies())
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试登录时密码错误的情况
func TestLoginInvalidPassword(t *testing.T) {
	passwordHash, err := argon2id.CreateHash("password", &argon2Params)
	require.NoError(t, err)
	userStore := new(mockUserStore)
	userStore.On("GetPasswordHash", mock.Anything, "alice").
		Return("user-id", passwordHash, nil)
	ssoStore := new(mockSSOStore)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.login(rr, newLoginRequest(`{"loginId":"alice","password":"wrong-password"}`))

	require.Equal(t, http.StatusUnauthorized, rr.Code, "body = %s", rr.Body.String())
	require.Empty(t, rr.Result().Cookies())
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}

// 测试登录成功的情况
func TestLoginSuccess(t *testing.T) {
	passwordHash, err := argon2id.CreateHash("password", &argon2Params)
	require.NoError(t, err)
	userStore := new(mockUserStore)
	userStore.On("GetPasswordHash", mock.Anything, "alice").
		Return("user-id", passwordHash, nil)
	ssoStore := new(mockSSOStore)
	ssoStore.On("Create", mock.Anything, "user-id").
		Return("session-id", nil)
	handler := newTestHandler(userStore, ssoStore)

	rr := httptest.NewRecorder()
	handler.login(rr, newLoginRequest(`{"loginId":"alice","password":"password"}`))

	cookies := rr.Result().Cookies()
	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	require.Len(t, cookies, 1)
	require.Equal(t, "session-id", cookies[0].Value)
	userStore.AssertExpectations(t)
	ssoStore.AssertExpectations(t)
}
