package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

// requireAuthorizeFieldViolation 是测试返回的 json 错误响应的 helper function
// 注意：不是帮助测试重定向产生错误响应
func requireAuthorizeFieldViolation(t *testing.T, resp *http.Response, field, description string) {
	t.Helper()

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "Invalid request parameters", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, field, violation["field"])
	require.Equal(t, description, violation["description"])
}

// requireRedirectToLogin 测试是否重定向到登录页面，return_to 参数是不是原始请求的 URL
func requireRedirectToLogin(t *testing.T, resp *http.Response, req *http.Request) {
	t.Helper()

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/login", location.Path)
	require.Equal(t, req.URL.RequestURI(), location.Query().Get("return_to"))
}

// TestAuthorizePKCESuccess 测试 OAuth 正常授权流程，返回授权码
func TestAuthorizePKCESuccess(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	ssoCookie := loginAndRegisterForAuthorizePKCE(t, "oauthuser")

	resp := doAuthorizePKCE(t, newAuthorizePKCERequest(t, clientID, testOAuthRedirectURI, ssoCookie, nil))

	require.Equal(t, http.StatusFound, resp.StatusCode)
	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https", location.Scheme)
	require.Equal(t, "client.example", location.Host)
	require.Equal(t, "/callback", location.Path)
	require.Equal(t, "value", location.Query().Get("existing"))

	// 测试返回的核心参数
	require.Equal(t, testOAuthState, location.Query().Get("state"))
	authCode := location.Query().Get("code")

	require.NotEmpty(t, authCode)

	key := "oauth:session:" + authCode
	storedSession, err := env.rdsClient.Get(t.Context(), key).Bytes()
	require.NoError(t, err)

	var oauthSession oauth.OAuthSession
	require.NoError(t, json.Unmarshal(storedSession, &oauthSession))
	require.Equal(t, clientID, oauthSession.ClientID)
	require.Equal(t, testOAuthRedirectURI, oauthSession.RedirectURI)
	require.Equal(t, codeChallengeFromVerifier(testOAuthCodeVerifier), oauthSession.CodeChallenge)
	require.NotEmpty(t, oauthSession.UserID)

	ttl, err := env.rdsClient.TTL(t.Context(), key).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, 10*time.Minute)
}

// TestAuthorizePKCERedirectsToLoginWithoutSSOCookie 测试没有 SSO cookie 的情况，应该重定向到登录页面
func TestAuthorizePKCERedirectsToLoginWithoutSSOCookie(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	req := newAuthorizePKCERequest(t, clientID, testOAuthRedirectURI, nil, nil)

	resp := doAuthorizePKCE(t, req)

	requireRedirectToLogin(t, resp, req)
}

// TestAuthorizePKCERedirectsToLoginWhenSSOCookieNotFound 测试 SSO cookie 在 Redis 中找不到的情况，应该重定向到登录页面
func TestAuthorizePKCERedirectsToLoginWhenSSOCookieNotFound(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	req := newAuthorizePKCERequest(
		t,
		clientID,
		testOAuthRedirectURI,
		&http.Cookie{
			Name:  sso.SSOCookieName,
			Value: "missing-session-id",
		},
		nil,
	)

	resp := doAuthorizePKCE(t, req)

	requireRedirectToLogin(t, resp, req)
}

// TestAuthorizePKCEUnknownClientDoesNotRedirect 测试未注册的 client_id 应当返回 json 错误响应
func TestAuthorizePKCEUnknownClientDoesNotRedirect(t *testing.T) {
	resetEnv(t)
	ssoCookie := loginAndRegisterForAuthorizePKCE(t, "oauthuser")

	resp := doAuthorizePKCE(t, newAuthorizePKCERequest(
		t,
		"00000000-0000-0000-0000-000000000000",
		testOAuthRedirectURI,
		ssoCookie,
		nil,
	))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// 不应该有重定向
	require.Empty(t, resp.Header.Get("Location"))

	// 测试返回的 JSON 错误响应
	requireAuthorizeFieldViolation(t, resp, "client_id", "Invalid client_id")
}

// TestAuthorizePKCERedirectURIMismatchDoesNotRedirect 测试不匹配的 redirect_uri 应当返回错误
func TestAuthorizePKCERedirectURIMismatchDoesNotRedirect(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	ssoCookie := loginAndRegisterForAuthorizePKCE(t, "oauthuser")

	resp := doAuthorizePKCE(t, newAuthorizePKCERequest(
		t,
		clientID,
		"https://attacker.example/callback",
		ssoCookie,
		nil,
	))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Empty(t, resp.Header.Get("Location"))
	requireAuthorizeFieldViolation(t, resp, "redirect_uri", "Invalid redirect_uri")
}

// TestAuthorizePKCEInvalidRequestRedirectsToRegisteredURI 测试重定向 URI 和 client ID 正确，但其他请求参数不合法的情况
func TestAuthorizePKCEInvalidRequestRedirectsToRegisteredURI(t *testing.T) {
	tests := []struct {
		name             string
		change           func(url.Values)
		errorDescription string
		expectState      string
	}{
		{
			name: "invalid response type",
			change: func(query url.Values) {
				// 设置为错的 response_type，RFC 6749 要求必须是 "code"
				query.Set("response_type", "token")
			},
			errorDescription: "response_type must be 'code'",
			expectState:      testOAuthState,
		},
		{
			name: "missing state",
			change: func(query url.Values) {
				query.Del("state")
			},
			errorDescription: "state is required",
		},
		{
			name: "missing code challenge",
			change: func(query url.Values) {
				query.Del("code_challenge")
			},
			errorDescription: "code_challenge is required",
			expectState:      testOAuthState,
		},
		{
			name: "unsupported code challenge method",
			change: func(query url.Values) {
				query.Set("code_challenge_method", "plain")
			},
			errorDescription: "code_challenge_method must be 'S256'",
			expectState:      testOAuthState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnv(t)
			clientID := registerOAuthClient(t, testOAuthRedirectURI)
			ssoCookie := loginAndRegisterForAuthorizePKCE(t, "oauthuser")

			resp := doAuthorizePKCE(t, newAuthorizePKCERequest(t, clientID, testOAuthRedirectURI, ssoCookie, tt.change))

			require.Equal(t, http.StatusFound, resp.StatusCode)
			location, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "https", location.Scheme)
			require.Equal(t, "client.example", location.Host)
			require.Equal(t, "/callback", location.Path)
			require.Equal(t, "value", location.Query().Get("existing"))
			require.Equal(t, "invalid_request", location.Query().Get("error"))
			require.Equal(t, tt.errorDescription, location.Query().Get("error_description"))
			require.Equal(t, tt.expectState, location.Query().Get("state"))
		})
	}
}
