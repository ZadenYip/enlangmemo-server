package oauthintegration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// newRevokeTokenRequest 会返回一个新的 OAuth 令牌撤销请求
func newRevokeTokenRequest(t *testing.T, form url.Values) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		suite.Server.URL+"/v1/oauth/revoke",
		strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

// doRevokeToken 发送令牌撤销请求并返回响应
func doRevokeToken(t *testing.T, form url.Values) *http.Response {
	t.Helper()

	resp, err := suite.Client.Do(newRevokeTokenRequest(t, form))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

func newRevokeTokenForm(clientID, accessToken string) url.Values {
	return url.Values{
		"client_id": {clientID},
		"token":     {accessToken},
	}
}

func exchangeAccessTokenForRevoke(t *testing.T, clientID, authCode string) string {
	t.Helper()

	form := newExchangeTokenForm(clientID, authCode, testOAuthRedirectURI, testOAuthCodeVerifier)
	resp := doExchangeToken(t, form)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body.AccessToken)

	return body.AccessToken
}

func requireRevokeError(t *testing.T, resp *http.Response, errorCode, description string) {
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

func requireAccessTokenExists(t *testing.T, accessToken string, wantExists int64) {
	t.Helper()

	exists, err := suite.Env.RDB.Exists(t.Context(), "oauth:access_token:"+accessToken).Result()
	require.NoError(t, err)
	require.Equal(t, wantExists, exists)
}

// TestRevokeAccessTokenSuccess 测试用正确的 client_id 撤销已存在的 access token
func TestRevokeAccessTokenSuccess(t *testing.T) {
	resetEnv(t)
	clientID, authCode := authorizePKCE(t, "revokeuser", testOAuthRedirectURI, nil)
	accessToken := exchangeAccessTokenForRevoke(t, clientID, authCode)

	resp := doRevokeToken(t, newRevokeTokenForm(clientID, accessToken))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireAccessTokenExists(t, accessToken, 0)
}

// TestRevokeAccessTokenRejectsUnknownClient 测试 client_id 对不上任何客户端时返回 invalid_client
func TestRevokeAccessTokenRejectsUnknownClient(t *testing.T) {
	resetEnv(t)

	resp := doRevokeToken(t, newRevokeTokenForm("unknown-client-id", "missing-access-token"))

	requireRevokeError(t, resp, "invalid_client", "Invalid client")
}

// TestRevokeAccessTokenIgnoresMissingToken 测试 client_id 正确但 access token 不存在时返回 200 OK
func TestRevokeAccessTokenIgnoresMissingToken(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)

	resp := doRevokeToken(t, newRevokeTokenForm(clientID, "missing-access-token"))

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestRevokeAccessTokenIgnoresClientMismatch 测试 token 存在但绑定的 client_id 不匹配时返回 200 OK，且不删除 token
func TestRevokeAccessTokenIgnoresClientMismatch(t *testing.T) {
	resetEnv(t)
	tokenClientID, authCode := authorizePKCE(t, "revokeuser", testOAuthRedirectURI, nil)
	accessToken := exchangeAccessTokenForRevoke(t, tokenClientID, authCode)
	otherClientID := registerOAuthClient(t, testOAuthRedirectURI)

	resp := doRevokeToken(t, newRevokeTokenForm(otherClientID, accessToken))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireAccessTokenExists(t, accessToken, 1)
}
