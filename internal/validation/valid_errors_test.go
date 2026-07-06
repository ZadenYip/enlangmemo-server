package validation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorError(t *testing.T) {
	rr := httptest.NewRecorder()

	v := NewValidator()
	v.FailMsg = "invalid request"
	v.AddFieldError("name", "invalid name")

	HandleValidationError(rr, v)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":400,"message":"invalid request","status":"INVALID_ARGUMENT","details":[{"fieldViolations":[{"field":"name","description":"invalid name"}]}]}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}

func TestUnexpectedError(t *testing.T) {
	rr := httptest.NewRecorder()

	HandleValidationError(rr, nil)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL","details":[]}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}
