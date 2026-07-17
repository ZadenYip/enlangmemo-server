package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newExchangeTokenRequest 会返回一个新的 OAuth 令牌交换请求
func newExchangeTokenRequest(t *testing.T, form url.Values) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		testServer.URL+"/v1/oauth/token",
		strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

// doExchangeToken 发送令牌交换请求并返回响应
func doExchangeToken(t *testing.T, form url.Values) *http.Response {
	t.Helper()

	resp, err := testClient.Do(newExchangeTokenRequest(t, form))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

// newExchangeTokenForm 创建请求表单数据
func newExchangeTokenForm(clientID, authCode, redirectURI, codeVerifier string) url.Values {
	return url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {authCode},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
}

// requireExchangeTokenError 是测试返回的 json 错误响应的 helper function
func requireExchangeTokenError(t *testing.T, resp *http.Response, errorCode, description string) {
	t.Helper()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, errorCode, body.Error)
	require.Equal(t, description, body.ErrorDescription)
}

// TestExchangeTokenSuccess 测试用正确的授权码和 code_verifier 成功兑换 access token
func TestExchangeTokenSuccess(t *testing.T) {
	resetEnv(t)
	clientID, authCode := authorizePKCE(t, "tokenuser", testOAuthRedirectURI, nil)
	form := newExchangeTokenForm(clientID, authCode, testOAuthRedirectURI, testOAuthCodeVerifier)

	resp := doExchangeToken(t, form)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	require.Equal(t, "no-cache", resp.Header.Get("Pragma"))

	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body.AccessToken)
	require.Equal(t, "bearer", body.TokenType)
	require.Equal(t, int64(3600*24), body.ExpiresIn)

	ttl, err := env.rdsClient.TTL(t.Context(), "oauth:access_token:"+body.AccessToken).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, 24*time.Hour)

	exists, err := env.rdsClient.Exists(t.Context(), "oauth:session:"+authCode).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}

// TestExchangeTokenRejectsEmptyForm 测试空表单数据的情况
func TestExchangeTokenRejectsEmptyForm(t *testing.T) {
	resetEnv(t)

	resp := doExchangeToken(t, url.Values{})

	requireExchangeTokenError(t, resp, "invalid_request", "grant_type is required")
}

// TestExchangeTokenRejectsMissingAuthorizationSession 测试授权码不存在或已过期的情况
func TestExchangeTokenRejectsMissingAuthorizationSession(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	form := newExchangeTokenForm(clientID, "missing-auth-code", testOAuthRedirectURI, testOAuthCodeVerifier)

	resp := doExchangeToken(t, form)

	requireExchangeTokenError(t, resp, "invalid_grant", "Invalid authorization code")
}

// TestExchangeTokenRejectsMismatchedCodeVerifier 测试 code_verifier 不匹配的情况
func TestExchangeTokenRejectsMismatchedCodeVerifier(t *testing.T) {
	resetEnv(t)
	clientID, authCode := authorizePKCE(t, "tokenuser", testOAuthRedirectURI, nil)
	form := newExchangeTokenForm(clientID, authCode, testOAuthRedirectURI, strings.Repeat("b", 43))

	resp := doExchangeToken(t, form)

	requireExchangeTokenError(t, resp, "invalid_grant", "code_verifier does not match the one used in the authorization request")
}

// TestExchangeTokenRejectsAuthorizationSessionBindingMismatch 测试授权会话绑定不匹配的情况
func TestExchangeTokenRejectsAuthorizationSessionBindingMismatch(t *testing.T) {
	tests := []struct {
		name        string
		change      func(url.Values)
		description string
	}{
		{
			name: "redirect uri mismatch",
			change: func(form url.Values) {
				// 设置为不匹配的 redirect_uri
				form.Set("redirect_uri", "https://client.example/other-callback")
			},
			description: "redirect_uri does not match the one used in the authorization request",
		},
		{
			name: "client id mismatch",
			change: func(form url.Values) {
				// 设置为不匹配的 client_id
				form.Set("client_id", "different-client-id")
			},
			description: "client_id does not match the one used in the authorization request",
		},
		{
			name: "code verifier mismatch",
			change: func(form url.Values) {
				// 设置为不匹配的 code_verifier
				form.Set("code_verifier", strings.Repeat("c", 43))
			},
			description: "code_verifier does not match the one used in the authorization request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnv(t)
			clientID, authCode := authorizePKCE(t, "tokenuser", testOAuthRedirectURI, nil)
			form := newExchangeTokenForm(clientID, authCode, testOAuthRedirectURI, testOAuthCodeVerifier)
			tt.change(form)

			resp := doExchangeToken(t, form)

			requireExchangeTokenError(t, resp, "invalid_grant", tt.description)
		})
	}
}
