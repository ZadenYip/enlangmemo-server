package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

func TestValidError(t *testing.T) {
	rr := httptest.NewRecorder()

	handleValidationError(rr, &validation.ValidError{
		FieldName: "name",
		Msg:       "invalid name",
	})

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":400,"message":"invalid name","status":"INVALID_ARGUMENT"}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}

func TestUnexpectedError(t *testing.T) {
	rr := httptest.NewRecorder()

	handleValidationError(rr, errors.New("boom"))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Equal(t,
		`{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL"}}`,
		strings.TrimSpace(rr.Body.String()),
	)
}
