package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newAppMeRequest(t *testing.T, accessToken string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		testServer.URL+"/v1/apps/enlangmemo/me",
		nil,
	)
	require.NoError(t, err)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	return req
}

func doAppMe(t *testing.T, accessToken string) *http.Response {
	t.Helper()

	resp, err := testClient.Do(newAppMeRequest(t, accessToken))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

// exchangeAccessToken 会执行一个完整的 OAuth 授权请求（包含注册用户、OAuth 客户端本身的注册、登录、授权请求）
func exchangeAccessToken(t *testing.T, loginID string) string {
	t.Helper()

	clientID, authCode := authorizePKCE(t, loginID, testOAuthRedirectURI, nil)
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

type appMeUser struct {
	UserID   string
	LoginID  string
	Nickname string
}

// userByLoginID 会根据 loginID（用户的登录用的ID）查询对应数据库用户信息
func userByLoginID(t *testing.T, loginID string) appMeUser {
	t.Helper()

	var userIDBytes []byte
	var actualLoginID string
	var nickname string
	err := env.db.QueryRowContext(t.Context(), `SELECT id, login_id, nickname FROM users WHERE login_id = ?`, loginID).
		Scan(&userIDBytes, &actualLoginID, &nickname)
	require.NoError(t, err)

	userID, err := uuid.FromBytes(userIDBytes)
	require.NoError(t, err)

	return appMeUser{
		UserID:   userID.String(),
		LoginID:  actualLoginID,
		Nickname: nickname,
	}
}

// TestAppMeReturnsUserProfile 测试 /v1/apps/enlangmemo/me 接口是否能正确返回当前登录用户信息
func TestAppMeReturnsUserProfile(t *testing.T) {
	resetEnv(t)
	loginID := "appmeuser"
	accessToken := exchangeAccessToken(t, loginID)
	expectedUser := userByLoginID(t, loginID)

	resp := doAppMe(t, accessToken)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body struct {
		UserID   string `json:"user_id"`
		LoginID  string `json:"login_id"`
		Nickname string `json:"nickname"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, expectedUser.UserID, body.UserID)
	require.Equal(t, expectedUser.LoginID, body.LoginID)
	require.Equal(t, expectedUser.Nickname, body.Nickname)
}

// TestAppMeRejectsMissingAccessToken 测试缺少 access token 时是否返回 401 Unauthorized
func TestAppMeRejectsMissingAccessToken(t *testing.T) {
	resetEnv(t)

	resp := doAppMe(t, "")

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestAppMeRejectsUnknownAccessToken 测试提供未知的 access token 时是否返回 401 Unauthorized
func TestAppMeRejectsUnknownAccessToken(t *testing.T) {
	resetEnv(t)

	resp := doAppMe(t, "missing-token")

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
