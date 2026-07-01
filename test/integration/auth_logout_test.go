package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

func newLogoutRequest(t *testing.T, cookie *http.Cookie) *http.Request {
	t.Helper()

	url := testServer.URL + "/v1/auth/logout"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
	require.NoError(t, err)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	return req
}

func doLogout(t *testing.T, cookie *http.Cookie) *http.Response {
	t.Helper()

	resp, err := testClient.Do(newLogoutRequest(t, cookie))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

func loginForLogout(t *testing.T) *http.Cookie {
	t.Helper()

	registerUserForLogin(t, "testuser", "testpassword")
	body := auth.LoginRequest{
		Name:     "testuser",
		Password: "testpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, len(resp.Cookies()), 1)

	return resp.Cookies()[0]
}

// TestLogoutSuccess 测试成功退出登录的情况
func TestLogoutSuccess(t *testing.T) {
	resetEnv(t)

	cookie := loginForLogout(t)
	resp := doLogout(t, cookie)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireExpiredSSOCookie(t, resp)

	var logoutResp auth.LogoutResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&logoutResp))
}

// 不带 cookie 的退出登录请求应该返回 200 OK，并清除 cookie
func TestLogoutMissingCookie(t *testing.T) {
	resetEnv(t)

	resp := doLogout(t, nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireExpiredSSOCookie(t, resp)

	var logoutResp auth.LogoutResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&logoutResp))
}

// 空 cookie 值应当当成未登录处理，并清除 cookie
func TestLogoutEmptyCookie(t *testing.T) {
	resetEnv(t)

	resp := doLogout(t, &http.Cookie{
		Name:  sso.CookieName,
		Value: "",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireExpiredSSOCookie(t, resp)

	var logoutResp auth.LogoutResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&logoutResp))
}

// TestLogoutInvalidOrExpiredCookie 测试带有无效或过期 cookie 的退出登录请求
func TestLogoutInvalidOrExpiredCookie(t *testing.T) {
	resetEnv(t)

	resp := doLogout(t, &http.Cookie{
		Name:  sso.CookieName,
		Value: "invalid-session-id",
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	requireExpiredSSOCookie(t, resp)

	var logoutResp auth.LogoutResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&logoutResp))
}

// 确定返回的 cookie 是一个过期的 SSO cookie
func requireExpiredSSOCookie(t *testing.T, resp *http.Response) {
	t.Helper()

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, sso.CookieName, cookies[0].Name)
	require.Empty(t, cookies[0].Value)
	require.Equal(t, -1, cookies[0].MaxAge)
}
