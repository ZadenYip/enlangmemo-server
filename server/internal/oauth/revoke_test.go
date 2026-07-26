package oauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testAccessToken = "access-token"

func newRevokeRequest(form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func validRevokeFormValues() url.Values {
	return url.Values{
		"client_id": {testClientID},
		"token":     {testAccessToken},
	}
}

func requireRevokeInvalidRequest(t *testing.T, rr *httptest.ResponseRecorder, description string) {
	t.Helper()

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	require.JSONEq(t, `{
		"error": "invalid_request",
		"error_description": `+description+`
	}`, rr.Body.String())
}

func TestRevokeAccessTokenSuccess(t *testing.T) {
	store := new(mockOAStore)
	store.On("RevokeAccessToken", mock.Anything, testAccessToken, testClientID).
		Return(nil).
		Once()

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).revoke(rr, newRevokeRequest(validRevokeFormValues()))

	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	store.AssertExpectations(t)
}

func TestRevokeUnknownAccessTokenReturnsOK(t *testing.T) {
	store := new(mockOAStore)
	store.On("RevokeAccessToken", mock.Anything, testAccessToken, testClientID).
		Return(ErrAccessTokenNotFound).
		Once()

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).revoke(rr, newRevokeRequest(validRevokeFormValues()))

	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	store.AssertExpectations(t)
}

func TestRevokeRejectsInvalidForm(t *testing.T) {
	tests := []struct {
		name            string
		change          func(url.Values)
		wantDescription string
	}{
		{
			name: "missing client id",
			change: func(form url.Values) {
				form.Del("client_id")
			},
			wantDescription: "client_id is required",
		},
		{
			name: "missing token",
			change: func(form url.Values) {
				form.Del("token")
			},
			wantDescription: "token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := new(mockOAStore)
			form := validRevokeFormValues()
			tt.change(form)

			rr := httptest.NewRecorder()
			newOAuthTestHandler(store).revoke(rr, newRevokeRequest(form))

			requireRevokeInvalidRequest(t, rr, `"`+tt.wantDescription+`"`)
			store.AssertNotCalled(t, "RevokeAccessToken", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestRevokeIgnoresTokenTypeHint(t *testing.T) {
	for _, hint := range []string{"unknown_token_type", "refresh_token"} {
		t.Run(hint, func(t *testing.T) {
			store := new(mockOAStore)
			store.On("RevokeAccessToken", mock.Anything, testAccessToken, testClientID).
				Return(nil).
				Once()

			form := validRevokeFormValues()
			form.Set("token_type_hint", hint)

			rr := httptest.NewRecorder()
			newOAuthTestHandler(store).revoke(rr, newRevokeRequest(form))

			require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
			store.AssertExpectations(t)
		})
	}
}

func TestRevokeTokenForDifferentClientReturnsOK(t *testing.T) {
	store := new(mockOAStore)
	store.On("RevokeAccessToken", mock.Anything, testAccessToken, testClientID).
		Return(errAccessTokenClientMismatch).
		Once()

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).revoke(rr, newRevokeRequest(validRevokeFormValues()))

	require.Equal(t, http.StatusOK, rr.Code, "body = %s", rr.Body.String())
	store.AssertExpectations(t)
}

func TestIsInvalidRevokeForm(t *testing.T) {
	tests := []struct {
		name            string
		form            revokeForm
		wantInvalid     bool
		wantDescription string
	}{
		{
			name: "missing client id",
			form: revokeForm{
				Token: testAccessToken,
			},
			wantInvalid:     true,
			wantDescription: "client_id is required",
		},
		{
			name: "missing token",
			form: revokeForm{
				ClientID: testClientID,
			},
			wantInvalid:     true,
			wantDescription: "token is required",
		},
		{
			name: "valid form",
			form: revokeForm{
				ClientID: testClientID,
				Token:    testAccessToken,
			},
			wantInvalid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid, description := isInvalidRevokeForm(tt.form)

			require.Equal(t, tt.wantInvalid, invalid)
			require.Equal(t, tt.wantDescription, description)
		})
	}
}
