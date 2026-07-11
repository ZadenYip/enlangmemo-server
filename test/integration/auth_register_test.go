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

func newRegisterRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()

	url := testServer.URL + "/v1/auth/register"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func doRegister(t *testing.T, body []byte) *http.Response {
	t.Helper()

	// 以下为响应
	resp, err := testClient.Do(newRegisterRequest(t, body))
	require.NoError(t, err)

	// 结束后关闭响应体
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	return resp
}

func marshalRegisterRequest(t *testing.T, body auth.RegisterRequest) []byte {
	t.Helper()

	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	return jsonBody
}

func TestRegisterSuccess(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	// 检查响应状态码
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var registerResp auth.RegisterResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&registerResp))
	require.NotEmpty(t, registerResp.UserID)
}

func TestRegisterJSONDecodeError(t *testing.T) {
	resetEnv(t)

	resp := doRegister(t, []byte(`{"loginId":"testuser","nickname":"测试用户","password":`))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "Request body contains badly-formed JSON", errResp.Error.Message)
}

func TestRegisterNameTooLong(t *testing.T) {
	resetEnv(t)

	// 超过最大长度的用户名（限制为16个字符）
	body := auth.RegisterRequest{
		// 17个字符
		LoginID:  "abcdefghijklmnopq",
		Nickname: "测试用户",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)

	require.Equal(t, "Invalid register request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	// 检查具体的 field violation
	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "loginId", violation["field"])
	require.Equal(t, "loginId must not be longer than 16 characters", violation["description"])
}

// 测试登录 ID 包含非法字符的情况（只允许英文字母和数字）
func TestRegisterLoginIDInvalidChars(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "test_user",
		Nickname: "测试用户",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "Invalid register request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "loginId", violation["field"])
	require.Equal(t, "loginId must contain only English letters and digits", violation["description"])
}

// 测试登录 ID 为空的情况
func TestRegisterLoginIDBlank(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "",
		Nickname: "测试用户",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, "Invalid register request", errResp.Error.Message)
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

// 测试昵称为空的情况
func TestRegisterNicknameBlank(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: " ",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, "Invalid register request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)

	detail, ok := errResp.Error.Details[0].(map[string]any)
	require.True(t, ok)
	violations, ok := detail["fieldViolations"].([]any)
	require.True(t, ok)
	require.Len(t, violations, 1)
	violation, ok := violations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "nickname", violation["field"])
	require.Equal(t, "nickname must not be blank", violation["description"])
}

// 测试密码为空的情况
func TestRegisterPasswordBlank(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		Password: "",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "Invalid register request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)
}

// 测试密码长度小于 8 个字符的情况
func TestRegisterPasswordTooShort(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		Password: "short",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, "Invalid register request", errResp.Error.Message)
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

// 测试密码允许 Unicode 字符和空格
func TestRegisterPasswordAllowsUnicodeAndWhitespace(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		Password: " 密码 abc ",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRegisterPasswordTooLong(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		// 33 字符
		Password: "abcdefghijklmnopqrstuvwxyzabcdefg",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	require.Equal(t, "Invalid register request", errResp.Error.Message)
	require.Len(t, errResp.Error.Details, 1)
}

func TestRegisterUserAlreadyExists(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		LoginID:  "testuser",
		Nickname: "测试用户",
		Password: "testpassword",
	}

	// 第一次注册应该成功
	firstResp := doRegister(t, marshalRegisterRequest(t, body))
	require.Equal(t, http.StatusCreated, firstResp.StatusCode)

	// 第二次注册应该返回用户已存在的错误
	secondResp := doRegister(t, marshalRegisterRequest(t, body))
	require.Equal(t, aip.StatusAlreadyExists.HTTPCode(), secondResp.StatusCode)
	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(secondResp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusAlreadyExists.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusAlreadyExists.String(), errResp.Error.Status)
	require.Equal(t, "User already exists", errResp.Error.Message)
}
