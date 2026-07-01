package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func newLoginRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()

	url := testServer.URL + "/v1/auth/login"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func doLogin(t *testing.T, body []byte) *http.Response {
	t.Helper()

	// 以下为响应
	resp, err := testClient.Do(newLoginRequest(t, body))
	require.NoError(t, err)

	// 结束后关闭响应体
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

func marshalLoginRequest(t *testing.T, body auth.LoginRequest) []byte {
	t.Helper()

	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	return jsonBody
}

func registerUserForLogin(t *testing.T, name string, password string) {
	t.Helper()

	body := auth.RegisterRequest{
		Name:     name,
		Password: password,
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestLoginSuccess(t *testing.T) {
	resetEnv(t)
	registerUserForLogin(t, "testuser", "testpassword")

	body := auth.LoginRequest{
		Name:     "testuser",
		Password: "testpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	// 状态码登录成功检查
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// 检查 Cookie 是否设置正确
	require.Equal(t, len(resp.Cookies()), 1)
	require.Equal(t, "__Host-sso_token", resp.Cookies()[0].Name)

	var loginResp auth.LoginResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginResp))
}

func TestLoginPasswordTooLong(t *testing.T) {
	resetEnv(t)

	body := auth.LoginRequest{
		Name: "testuser",
		// 33 字符
		Password: "abcdefghijklmnopqrstuvwxyzabcdefg",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "password must not be longer than 32 characters", errResp.Error.Message)
}

func TestLoginUserNotFound(t *testing.T) {
	resetEnv(t)

	body := auth.LoginRequest{
		Name:     "missinguser",
		Password: "testpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusNotFound.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusNotFound.String(), errResp.Error.Status)
	require.Equal(t, "User not found", errResp.Error.Message)
}

func TestLoginInvalidPassword(t *testing.T) {
	resetEnv(t)
	registerUserForLogin(t, "testuser", "testpassword")

	body := auth.LoginRequest{
		Name:     "testuser",
		Password: "wrongpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusUnauthenticated.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusUnauthenticated.String(), errResp.Error.Status)
	require.Equal(t, "Invalid password", errResp.Error.Message)
}
