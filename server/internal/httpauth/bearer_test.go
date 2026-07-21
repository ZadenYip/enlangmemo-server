package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{
			name:      "valid bearer token",
			header:    "Bearer access-token",
			wantToken: "access-token",
			wantOK:    true,
		},
		{
			name:      "case insensitive scheme",
			header:    "bearer access-token",
			wantToken: "access-token",
			wantOK:    true,
		},
		{
			name:   "missing header",
			wantOK: false,
		},
		{
			name:   "unsupported scheme",
			header: "Basic access-token",
			wantOK: false,
		},
		{
			name:   "missing token",
			header: "Bearer",
			wantOK: false,
		},
		{
			name:   "too many fields",
			header: "Bearer access token",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, ok := BearerToken(req)

			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantToken, token)
		})
	}
}
