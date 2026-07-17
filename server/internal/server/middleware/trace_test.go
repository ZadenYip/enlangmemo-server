package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// TestTraceAddsMissingTraceHeader 测试 Trace 中间件会为缺失的 Trace Header 添加一个新的 Trace ID
func TestTraceAddsMissingTraceHeader(t *testing.T) {
	var capturedReq *http.Request

	// 创建一个测试的 HTTP handler，用于捕获请求并检查 Trace Header
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusNoContent)
	})
	handler := Trace(next, logging.NewServerLog())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.NotNil(t, capturedReq)

	trace := capturedReq.Header.Get(TraceHeader)
	require.NotEmpty(t, trace)
	_, err := uuid.Parse(trace)
	require.NoError(t, err)

	traceFromCtx, ok := logging.TraceIDFromCtx(capturedReq.Context())
	require.True(t, ok)
	require.Equal(t, trace, traceFromCtx)
}
