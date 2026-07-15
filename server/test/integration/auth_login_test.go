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

func registerUserForLogin(t *testing.T, loginID string, password string) {
	t.Helper()

	body := auth.RegisterRequest{
		LoginID:  loginID,
		Nickname: "测试用户",
		Password: password,
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestLoginSuccess(t *testing.T) {
	resetEnv(t)
	registerUserForLogin(t, "testuser", "testpassword")

	body := auth.LoginRequest{
		LoginID:  "testuser",
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
		LoginID: "testuser",
		// 17 字符
		Password: "abcdefghijklmnopq",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "invalid login request", errResp.Error.Message)

	// 检查具体的 field violation
	require.Len(t, errResp.Error.Details, 1)
	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "password", violation["field"])
	require.Equal(t, "password must not be longer than 16 characters", violation["description"])
}

func TestLoginLoginIDBlank(t *testing.T) {
	resetEnv(t)

	body := auth.LoginRequest{
		LoginID:  "",
		Password: "testpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, "invalid login request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "loginId", violation["field"])
	require.Equal(t, "loginId must not be blank", violation["description"])
}

func TestLoginPasswordTooShort(t *testing.T) {
	resetEnv(t)

	body := auth.LoginRequest{
		LoginID:  "testuser",
		Password: "short",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, "invalid login request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "password", violation["field"])
	require.Equal(t, "password must be at least 8 characters", violation["description"])
}

func TestLoginUserNotFound(t *testing.T) {
	resetEnv(t)

	body := auth.LoginRequest{
		LoginID:  "missinguser",
		Password: "testpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusUnauthenticated.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusUnauthenticated.String(), errResp.Error.Status)
	require.Equal(t, "invalid login credentials", errResp.Error.Message)
}

func TestLoginInvalidPassword(t *testing.T) {
	resetEnv(t)
	registerUserForLogin(t, "testuser", "testpassword")

	body := auth.LoginRequest{
		LoginID:  "testuser",
		Password: "wrongpassword",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusUnauthenticated.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusUnauthenticated.String(), errResp.Error.Status)
	require.Equal(t, "invalid login credentials", errResp.Error.Message)
}

func TestLoginBlankPassword(t *testing.T) {
	resetEnv(t)
	registerUserForLogin(t, "testuser", "testpassword")

	body := auth.LoginRequest{
		LoginID:  "testuser",
		Password: "",
	}
	resp := doLogin(t, marshalLoginRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "invalid login request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)
}
