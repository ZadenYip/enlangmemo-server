package httpjson

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

// 测试 DecodeJSONBody 函数在 Content-Type 不是 application/json 时返回错误
func TestDecodeJSONBodyInvalidContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"Alice"}`))
	req.Header.Set("Content-Type", "text/plain")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Equal(t, "Content-Type header is not application/json", err.Error())
}

// 测试 DecodeJSONBody 函数在 body 是非法 JSON 时返回语法错误
func TestDecodeJSONBodySyntaxError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":}`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Contains(t, err.Error(), "Request body contains badly-formed JSON")
}

// 测试 DecodeJSONBody 函数在字段类型不匹配时返回错误
func TestDecodeJSONBodyUnmarshalTypeError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"age":"old"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Age int `json:"age"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Contains(t, err.Error(), `Request body contains an invalid value for the "age" field`)
}

// 测试 DecodeJSONBody 函数在遇到未知字段时返回错误
func TestDecodeJSONBodyUnknownField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"unknown":"value"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Equal(t, `Request body contains unknown field "unknown"`, err.Error())
}

// 测试 DecodeJSONBody 函数在 body 为空时返回错误
func TestDecodeJSONBodyEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Equal(t, "Request body must not be empty", err.Error())
}

// 测试 DecodeJSONBody 函数在 body 超过 1MB 时返回错误
func TestDecodeJSONBodyMaxBytesError(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", 1<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Equal(t, "Request body must not be larger than 1048576 bytes", err.Error())
}

// 测试 DecodeJSONBody 函数在遇到非 JSON 解码错误应该返回原始错误
func TestDecodeJSONBodyUnexpectedDecodeError(t *testing.T) {
	errMockRead := errors.New("mock read error")
	// iotest.ErrReader 会在读取时返回指定的错误
	req := httptest.NewRequest(http.MethodPost, "/test", iotest.ErrReader(errMockRead))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.ErrorIs(t, err, errMockRead)
}

// 测试 DecodeJSONBody 函数在 body 有多个 JSON 对象时返回错误
func TestDecodeJSONBodyMultipleJSONObjects(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"Alice"}{"name":"Bob"}`))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := DecodeJSONBody(httptest.NewRecorder(), req, &dst)

	require.Error(t, err)
	var malformed *malformedRequest
	require.True(t, errors.As(err, &malformed))
	require.Equal(t, "Request body must only contain a single JSON object", err.Error())
}

// 测试无法 marshal 为 json 应当返回 500 错误
func TestResponseJSONMarshalError(t *testing.T) {
	rr := httptest.NewRecorder()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// channel 类型没法 marshal 为 json
	ResponseJSON(rr, http.StatusOK, make(chan int), logger)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.JSONEq(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL"}}`,
		rr.Body.String(),
	)
}

// HandleJSONDecodeError 函数在遇到意外错误应该返回 500 错误
func TestHandleJSONDecodeErrorUnexpectedError(t *testing.T) {
	rr := httptest.NewRecorder()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// mock 一个意外的 decode 错误
	HandleJSONDecodeError(rr, errors.New("unexpected decode error(mocked)"), logger)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.JSONEq(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL","details":[]}}`,
		rr.Body.String(),
	)
}
