package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

func TestHandleValidationError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name: "validation error",
			err: &validation.ValidError{
				FieldName: "name",
				Msg:       "invalid name",
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":400,"message":"invalid name","status":"INVALID_ARGUMENT"}}`,
		},
		{
			name:       "unexpected error",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":{"code":500,"message":"Internal Server Error","status":"INTERNAL"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			handleValidationError(rr, tt.err)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if got := rr.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", got, "application/json")
			}

			if got := strings.TrimSpace(rr.Body.String()); got != tt.wantBody {
				t.Fatalf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}
