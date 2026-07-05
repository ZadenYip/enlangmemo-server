package validation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidError(t *testing.T) {
	rr := httptest.NewRecorder()

	HandleValidationError(rr, &ValidError{
		FieldName: "name",
		Msg:       "invalid name",
	})

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":400,"message":"invalid name","status":"INVALID_ARGUMENT","details":[]}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}

func TestUnexpectedError(t *testing.T) {
	rr := httptest.NewRecorder()

	HandleValidationError(rr, errors.New("boom"))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL","details":[]}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}
