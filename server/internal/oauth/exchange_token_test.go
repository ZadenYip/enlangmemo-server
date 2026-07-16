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

const (
	testAuthCode     = "authorization-code"
	testCodeVerifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
)

func newExchangeTokenRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/oauth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func validTokenFormData() tokenFormData {
	return tokenFormData{
		grantType:    "authorization_code",
		code:         testAuthCode,
		redirectURI:  testRedirectURI,
		clientID:     testClientID,
		codeVerifier: testCodeVerifier,
	}
}

func validTokenFormValues() url.Values {
	formData := validTokenFormData()
	return url.Values{
		"grant_type":    {formData.grantType},
		"code":          {formData.code},
		"redirect_uri":  {formData.redirectURI},
		"client_id":     {formData.clientID},
		"code_verifier": {formData.codeVerifier},
	}
}

func requireTokenError(t *testing.T, rr *httptest.ResponseRecorder, description string) {
	t.Helper()

	require.Equal(t, http.StatusBadRequest, rr.Code, "body = %s", rr.Body.String())
	require.JSONEq(t, `{
		"error": "invalid_request",
		"error_description": `+description+`
	}`, rr.Body.String())
}

// TestExchangeTokenWithoutFormDataReturnsInvalidRequest 测试没有表单数据的请求是否返回无效请求错误
func TestExchangeTokenWithoutFormDataReturnsInvalidRequest(t *testing.T) {
	store := new(mockOAStore)
	rr := httptest.NewRecorder()

	newOAuthTestHandler(store).exchangeToken(rr, httptest.NewRequest(http.MethodPost, "/v1/oauth/token", nil))

	requireTokenError(t, rr, `"grant_type is required"`)
	store.AssertNotCalled(t, "ConsumeCodeSession", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "GenAccessToken", mock.Anything, mock.Anything)
}

// TestExchangeTokenWithNonUTF8FormDataReturnsInvalidRequest 测试包含非 UTF-8 编码的表单数据的请求是否返回无效请求错误
func TestExchangeTokenWithNonUTF8FormDataReturnsInvalidRequest(t *testing.T) {
	store := new(mockOAStore)
	form := validTokenFormValues()
	form.Del("grant_type")

	rr := httptest.NewRecorder()
	newOAuthTestHandler(store).exchangeToken(rr, newExchangeTokenRequest("grant_type=%ff&"+form.Encode()))

	requireTokenError(t, rr, `"Invalid UTF-8 encoding in form data"`)
	store.AssertNotCalled(t, "ConsumeCodeSession", mock.Anything, mock.Anything)
	store.AssertNotCalled(t, "GenAccessToken", mock.Anything, mock.Anything)
}

func TestIsInvalidForm(t *testing.T) {
	tests := []struct {
		name            string
		change          func(*tokenFormData)
		wantInvalid     bool
		wantDescription string
	}{
		{
			name: "non utf8 grant type",
			change: func(formData *tokenFormData) {
				formData.grantType = string([]byte{0xff})
			},
			wantInvalid:     true,
			wantDescription: "Invalid UTF-8 encoding in form data",
		},
		{
			name: "missing grant type",
			change: func(formData *tokenFormData) {
				formData.grantType = ""
			},
			wantInvalid:     true,
			wantDescription: "grant_type is required",
		},
		{
			name: "unsupported grant type",
			change: func(formData *tokenFormData) {
				formData.grantType = "refresh_token"
			},
			wantInvalid:     true,
			wantDescription: "grant_type must be 'authorization_code'",
		},
		{
			name: "missing code",
			change: func(formData *tokenFormData) {
				formData.code = ""
			},
			wantInvalid:     true,
			wantDescription: "code is required",
		},
		{
			name: "missing redirect uri",
			change: func(formData *tokenFormData) {
				formData.redirectURI = ""
			},
			wantInvalid:     true,
			wantDescription: "redirect_uri is required",
		},
		{
			name: "missing client id",
			change: func(formData *tokenFormData) {
				formData.clientID = ""
			},
			wantInvalid:     true,
			wantDescription: "client_id is required",
		},
		{
			name: "missing code verifier",
			change: func(formData *tokenFormData) {
				formData.codeVerifier = ""
			},
			wantInvalid:     true,
			wantDescription: "code_verifier is required",
		},
		{
			name: "invalid code verifier",
			change: func(formData *tokenFormData) {
				formData.codeVerifier = "short"
			},
			wantInvalid:     true,
			wantDescription: "code_verifier must be between 43 and 128 characters and contain only unreserved characters",
		},
		{
			name:        "valid form",
			change:      func(*tokenFormData) {},
			wantInvalid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formData := validTokenFormData()
			tt.change(&formData)

			invalid, description := isInvalidForm(formData)

			require.Equal(t, tt.wantInvalid, invalid)
			require.Equal(t, tt.wantDescription, description)
		})
	}
}

func TestIsInValidCodeVerifier(t *testing.T) {
	tests := []struct {
		name         string
		codeVerifier string
		wantInvalid  bool
	}{
		{
			name:         "too short",
			codeVerifier: strings.Repeat("a", 42),
			wantInvalid:  true,
		},
		{
			name:         "too long",
			codeVerifier: strings.Repeat("a", 129),
			wantInvalid:  true,
		},
		{
			name:         "contains reserved character",
			codeVerifier: strings.Repeat("a", 42) + "/",
			wantInvalid:  true,
		},
		{
			name:         "minimum length with unreserved characters",
			codeVerifier: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
			wantInvalid:  false,
		},
		{
			name:         "maximum length",
			codeVerifier: strings.Repeat("a", 128),
			wantInvalid:  false,
		},
		{
			name:         "all unreserved character types",
			codeVerifier: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~",
			wantInvalid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantInvalid, isInValidCodeVerifier(tt.codeVerifier))
		})
	}
}
