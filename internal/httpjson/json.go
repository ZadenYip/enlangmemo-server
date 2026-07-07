package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
)

type malformedRequest struct {
	msg string
}

type ErrResponse = aip.ErrResponse
type ErrInfo = aip.ErrStatus

func (mr *malformedRequest) Error() string {
	return mr.msg
}

// 几乎实现是 https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	contType := r.Header.Get("Content-Type")
	if contType != "" {
		// 获得第一个分号前的部分，因为可能会有 charset=utf-8 之类的参数
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contType, ";")[0]))
		if mediaType != "application/json" {
			msg := "Content-Type header is not application/json"
			return &malformedRequest{msg: msg}
		}
	}

	// 1MB
	const maxBytes = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &malformedRequest{msg: msg}

		// 没解析到末尾就遇到了 EOF，JSON 有问题
		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			return &malformedRequest{msg: msg}

		case errors.As(err, &unmarshalTypeError):
			const format = "Request body contains an invalid value for the %q field (at position %d)"
			msg := fmt.Sprintf(format, unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &malformedRequest{msg: msg}

		// 错误有前缀 "json: unknown field "，说明 JSON 中有未定义的字段
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &malformedRequest{msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &malformedRequest{msg: msg}

		case errors.As(err, &maxBytesError):
			msg := fmt.Sprintf("Request body must not be larger than %d bytes", maxBytes)
			return &malformedRequest{msg: msg}

		default:
			return err
		}
	}

	// 检测 JSON 有没有第二个对象
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		msg := "Request body must only contain a single JSON object"
		return &malformedRequest{msg: msg}
	}

	return nil
}

// 处理 JSON 解码错误
func HandleJSONDecodeError(w http.ResponseWriter, err error, errLog *slog.Logger) {
	var mr *malformedRequest
	if errors.As(err, &mr) {
		ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInvalidArgument).
				WithMessage(mr.msg),
			errLog)
		return
	}

	errLog.Error("unexpected json decode error", "err", err)
	ResponseError(w,
		aip.NewErrResponse().
			WithCodeAndStatus(aip.StatusInternal).
			WithMessage(http.StatusText(aip.StatusInternal.HTTPCode())),
		errLog)
}

// 处理验证错误
// json 遵循 Google 的 AIP
// status - 参考 https://cloud.google.com/apis/design/errors#error_responses 中的 status 字段
func ResponseError(w http.ResponseWriter, errResp *aip.ErrResponse, errLog *slog.Logger) {
	ResponseJSON(w, errResp.Error.Code, errResp, errLog)
}

// 返回 JSON 响应
func ResponseJSON(w http.ResponseWriter, httpStatus int, v any, errLog *slog.Logger) {
	js, err := json.Marshal(v)
	if err != nil {
		errLog.Error("failed to marshal json response", "err", err)

		const status = http.StatusInternalServerError
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		const internalErrJS = `{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL"}}`
		_, err = w.Write([]byte(internalErrJS))

		errLog.Error("failed to write internal error json response", "err", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_, _ = w.Write(js)
}
