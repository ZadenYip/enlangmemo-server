package oauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

const (
	testClientID      = "client-id"
	testRedirectURI   = "https://client.example/callback?source=test"
	testState         = "state-value"
	testCodeChallenge = "0123456789012345678901234567890123456789012"
)

type mockOAStore struct {
	mock.Mock
}

func (s *mockOAStore) GetClientInfo(ctx context.Context, clientID string) (OAClientInfo, error) {
	args := s.Called(ctx, clientID)
	return args.Get(0).(OAClientInfo), args.Error(1)
}

func (s *mockOAStore) GenCodeStoreSession(ctx context.Context, info AuthorizationInfo) (string, error) {
	args := s.Called(ctx, info)
	return args.String(0), args.Error(1)
}

// newAuthorizeRequest 是创建 OAuth 授权请求的 helper function
//
// change 是用来修改查询参数，方便测试缺少某个参数或参数值不对的情况
func newAuthorizeRequest(change func(url.Values)) *http.Request {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {testClientID},
		"redirect_uri":          {testRedirectURI},
		"state":                 {testState},
		"code_challenge":        {testCodeChallenge},
		"code_challenge_method": {"S256"},
	}
	if change != nil {
		change(query)
	}

	return httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil)
}

func newOAuthTestHandler(store OAStorer) *OAuthHandler {
	return NewOAuthHandler(store, logging.NewServerLog())
}

// 正常授权测试
func TestAuthorizeSuccess(t *testing.T) {
	store := new(mockOAStore)
	expectedInfo := AuthorizationInfo{
		responseType:        "code",
		clientID:            testClientID,
		redirectURI:         testRedirectURI,
		state:               testState,
		codeChallenge:       testCodeChallenge,
		codeChallengeMethod: "S256",
	}
	store.On("GetClientInfo", mock.Anything, testClientID).
		Return(OAClientInfo{ClientID: testClientID, RedirectURI: testRedirectURI}, nil).
		Once()
	store.On("GenCodeStoreSession", mock.Anything, expectedInfo).
		Return("authorization-code", nil).
		Once()

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).authorize(rr, newAuthorizeRequest(nil))

	require.Equal(t, http.StatusFound, rr.Code, "body = %s", rr.Body.String())
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https", location.Scheme)
	require.Equal(t, "client.example", location.Host)
	require.Equal(t, "/callback", location.Path)
	require.Equal(t, "test", location.Query().Get("source"))
	require.Equal(t, "authorization-code", location.Query().Get("code"))
	require.Equal(t, testState, location.Query().Get("state"))
	store.AssertExpectations(t)
}

// 测试缺少 client_id 参数的情况
func TestAuthorizeMissingClientIDReturnsJSONError(t *testing.T) {
	store := new(mockOAStore)
	rr := httptest.NewRecorder()
	req := newAuthorizeRequest(func(query url.Values) {
		// 删除 client_id 参数
		query.Del("client_id")
	})

	newOAuthTestHandler(store).authorize(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.JSONEq(t, `{
		"error": {
			"code": 400,
			"message": "Invalid request parameters",
			"status": "INVALID_ARGUMENT",
			"details": [{
				"fieldViolations": [{
					"field": "client_id",
					"description": "client_id is required"
				}]
			}]
		}
	}`, rr.Body.String())
	store.AssertNotCalled(t, "GetClientInfo", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "GenCodeStoreSession", mock.Anything, mock.Anything)
}

// 测试未知的 client_id 返回 JSON 错误
func TestAuthorizeUnknownClientReturnsJSONError(t *testing.T) {
	store := new(mockOAStore)

	// mock GetClientInfo 不知道这个 client_id
	store.On("GetClientInfo", mock.Anything, testClientID).
		Return(OAClientInfo{}, ErrOAClientNotFound).
		Once()

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).authorize(rr, newAuthorizeRequest(nil))

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	require.Contains(t, rr.Body.String(), `"field":"client_id"`)
	require.Contains(t, rr.Body.String(), `"description":"Invalid client_id"`)
	store.AssertNotCalled(t, "GenCodeStoreSession", mock.Anything, mock.Anything)
	store.AssertExpectations(t)
}

// 测试重定向的 URI 与注册的 redirect_uri 不一致的情况
func TestAuthorizeMismatchedRedirectURIReturnsJSONError(t *testing.T) {
	store := new(mockOAStore)
	store.On("GetClientInfo", mock.Anything, testClientID).
		Return(OAClientInfo{ClientID: testClientID, RedirectURI: testRedirectURI}, nil).
		Once()

	req := newAuthorizeRequest(func(query url.Values) {
		// 把重定向的 URI 改成不合法的 URI
		query.Set("redirect_uri", "https://attacker.example/callback")
	})

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).authorize(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	require.Empty(t, rr.Header().Get("Location"))
	require.Contains(t, rr.Body.String(), `"field":"redirect_uri"`)
	require.Contains(t, rr.Body.String(), `"description":"Invalid redirect_uri"`)
	store.AssertNotCalled(t, "GenCodeStoreSession", mock.Anything, mock.Anything)
	store.AssertExpectations(t)
}

// 测试一些在 PKCE 下不合法的请求参数
func TestAuthorizeInvalidPKCERequestRedirectsWithError(t *testing.T) {
	tests := []struct {
		name             string
		change           func(url.Values)
		errorDescription string
		expectState      string
	}{
		{
			name: "invalid response type",
			change: func(query url.Values) {
				// 设置非法的 response_type，根据 RFC 文档正确的值是 "code"
				query.Set("response_type", "token")
			},
			errorDescription: "response_type must be 'code'",
			expectState:      testState,
		},
		{
			name: "missing state",
			change: func(query url.Values) {
				// 删除 state 参数，实际实现要求客户端必须传 state 参数（尽管可选）
				query.Del("state")
			},
			errorDescription: "state is required",
		},
		{
			name: "missing code challenge",
			change: func(query url.Values) {
				// 删除 code_challenge 参数，文档要求 PKCE 必须传这个参数
				query.Del("code_challenge")
			},
			errorDescription: "code_challenge is required",
			expectState:      testState,
		},
		{
			name: "unsupported code challenge method",
			change: func(query url.Values) {
				// 实现没有支持 plain 方法，要求必须是 S256
				query.Set("code_challenge_method", "plain")
			},
			errorDescription: "code_challenge_method must be 'S256'",
			expectState:      testState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := new(mockOAStore)
			store.On("GetClientInfo", mock.Anything, testClientID).
				Return(OAClientInfo{ClientID: testClientID, RedirectURI: testRedirectURI}, nil).
				Once()

			rr := httptest.NewRecorder()
			newOAuthTestHandler(store).authorize(rr, newAuthorizeRequest(tt.change))

			require.Equal(t, http.StatusFound, rr.Code, "body = %s", rr.Body.String())
			location, err := url.Parse(rr.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "https", location.Scheme)
			require.Equal(t, "client.example", location.Host)
			require.Equal(t, "/callback", location.Path)
			require.Equal(t, "test", location.Query().Get("source"))
			require.Equal(t, "invalid_request", location.Query().Get("error"))
			require.Equal(t, tt.errorDescription, location.Query().Get("error_description"))
			require.Equal(t, tt.expectState, location.Query().Get("state"))
			store.AssertNotCalled(t, "GenCodeStoreSession", mock.Anything, mock.Anything)
			store.AssertExpectations(t)
		})
	}
}

func TestAuthorizeStoreErrorsReturnInternalError(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mockOAStore)
	}{
		{
			name: "get client info",
			setup: func(store *mockOAStore) {
				store.On("GetClientInfo", mock.Anything, testClientID).
					Return(OAClientInfo{}, errors.New("database unavailable")).
					Once()
			},
		},
		{
			name: "generate authorization code",
			setup: func(store *mockOAStore) {
				store.On("GetClientInfo", mock.Anything, testClientID).
					Return(OAClientInfo{ClientID: testClientID, RedirectURI: testRedirectURI}, nil).
					Once()
				store.On("GenCodeStoreSession", mock.Anything, mock.Anything).
					Return("", errors.New("redis unavailable")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := new(mockOAStore)
			tt.setup(store)

			rr := httptest.NewRecorder()
			newOAuthTestHandler(store).authorize(rr, newAuthorizeRequest(nil))

			require.Equal(t, http.StatusInternalServerError, rr.Code, "body = %s", rr.Body.String())
			require.JSONEq(t, `{
				"error": {
					"code": 500,
					"message": "Internal server error",
					"status": "INTERNAL",
					"details": []
				}
			}`, rr.Body.String())
			store.AssertExpectations(t)
		})
	}
}
