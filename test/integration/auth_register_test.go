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
		Name:     "testuser",
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

	resp := doRegister(t, []byte(`{"name":"testuser","password":`))

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
		Name:     "abcdefghijklmnopq",
		Password: "testpassword",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)

	// validation.go
	// fmt.Sprintf("%s must not be longer than %d characters", fieldName, maxLen)
	require.Equal(t, "name must not be longer than 16 characters", errResp.Error.Message)
}

func TestRegisterPasswordTooLong(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		Name: "testuser",
		// 33 字符
		Password: "abcdefghijklmnopqrstuvwxyzabcdefg",
	}
	resp := doRegister(t, marshalRegisterRequest(t, body))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp httpjson.ErrResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, aip.StatusInvalidArgument.HTTPCode(), errResp.Error.Code)
	require.Equal(t, aip.StatusInvalidArgument.String(), errResp.Error.Status)
	// validation.go
	// fmt.Sprintf("%s must not be longer than %d characters", fieldName, maxLen)
	require.Equal(t, "password must not be longer than 32 characters", errResp.Error.Message)
}

func TestRegisterUserAlreadyExists(t *testing.T) {
	resetEnv(t)

	body := auth.RegisterRequest{
		Name:     "testuser",
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
