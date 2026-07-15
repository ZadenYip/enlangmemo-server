package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

func TestPanicRecoveryRecoveredPanic(t *testing.T) {
	// 添加中间件测试 PanicRecovery 中的 recover 机制
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := PanicRecovery(next, logging.NewServerLog())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.JSONEq(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL","details":[]}}`,
		rr.Body.String(),
	)
}
