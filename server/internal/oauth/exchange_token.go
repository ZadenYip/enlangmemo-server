package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

func (h *OAuthHandler) exchangeToken(w http.ResponseWriter, r *http.Request) {
	formData, ok := h.extractFormData(w, r)
	if !ok {
		return
	}

	// 验证请求参数是否有效
	if invalid, description := isInvalidForm(formData); invalid {
		h.responseExchangeErr(w, exInvalidRequest, description)
		return
	}

	session, err := h.oaStore.ConsumeCodeSession(r.Context(), formData.code)
	switch {
	case errors.Is(err, errOASessionNotFound):
		h.responseExchangeErr(w, exInvalidGrant, "Invalid authorization code")
		return
	case err != nil:
		h.log.ErrorCtx(r.Context(), "failed to get oauth session", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Internal server error", h.log.Error())
		return
	}

	// 验证 code 与 session 的绑定关系
	if invalid, description := invalidCodeBinding(formData, session); invalid {
		h.responseExchangeErr(w, exInvalidGrant, description)
		return
	}

	h.responseWithAccessToken(w, r, session)
}

// https://datatracker.ietf.org/doc/html/rfc6749#section-5.1
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	// token_type 见 https://datatracker.ietf.org/doc/html/rfc6749#section-7.1
	TokenType string `json:"token_type"`
	ExpiresIn int64  `json:"expires_in"`
}

func (h *OAuthHandler) responseWithAccessToken(w http.ResponseWriter, r *http.Request, session OAuthSession) {

	token, err := h.oaStore.GenAccessToken(r.Context(), session.ClientID)

	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to generate access token", "err", err)
		httpjson.ResponseJSON(w, http.StatusInternalServerError, tokenErrorResponse{
			Error:            string(authorServerError),
			ErrorDescription: "Internal server error",
		}, h.log.Error())
		return
	}

	tokenResponse := tokenResponse{
		AccessToken: token,
		TokenType:   "bearer",
		ExpiresIn:   3600 * accessTokenTTLHours,
	}

	// RFC 6749 5.1 要求设置下面两个 Header
	r.Header.Set("Cache-Control", "no-store")
	r.Header.Set("Pragma", "no-cache")

	httpjson.ResponseJSON(w, http.StatusOK, tokenResponse, h.log.Error())
}

type tokenFormData struct {
	grantType    string
	code         string
	redirectURI  string
	clientID     string
	codeVerifier string
}

func (h *OAuthHandler) extractFormData(w http.ResponseWriter, r *http.Request) (tokenFormData, bool) {
	err := r.ParseForm()
	if err != nil {
		return tokenFormData{}, false
	}

	form := r.PostForm
	if form == nil {
		h.log.InfoCtx(r.Context(), "No form data found in the request")
		h.responseExchangeErr(w, authorInvalidRequest, "No form data found in the request")
		return tokenFormData{}, false
	}

	formData := tokenFormData{
		grantType:    form.Get("grant_type"),
		code:         form.Get("code"),
		redirectURI:  form.Get("redirect_uri"),
		clientID:     form.Get("client_id"),
		codeVerifier: form.Get("code_verifier"),
	}

	return formData, true
}

func isInvalidForm(formData tokenFormData) (invalid bool, description string) {
	// 查看是否为 UTF-8 编码
	if !utf8.ValidString(formData.grantType) ||
		!utf8.ValidString(formData.code) ||
		!utf8.ValidString(formData.redirectURI) ||
		!utf8.ValidString(formData.clientID) ||
		!utf8.ValidString(formData.codeVerifier) {
		return true, "Invalid UTF-8 encoding in form data"
	}

	switch {
	case formData.grantType == "":
		return true, "grant_type is required"
	case formData.grantType != "authorization_code":
		return true, "grant_type must be 'authorization_code'"
	case formData.code == "":
		return true, "code is required"
	case formData.redirectURI == "":
		return true, "redirect_uri is required"
	case formData.clientID == "":
		return true, "client_id is required"
	case formData.codeVerifier == "":
		return true, "code_verifier is required"
	case isInValidCodeVerifier(formData.codeVerifier):
		return true, "code_verifier must be between 43 and 128 characters and contain only unreserved characters"
	}

	return false, ""
}

// isInValidCodeVerifier 验证 code_verifier 是否符合 RFC 7636 Section 4.1 的要求
// code_verifier 的长度必须在 43 到 128 个字符之间，并且只能包含 unreserved characters RFC 3986
// 见 RFC 3986 Section 2.3: https://datatracker.ietf.org/doc/html/rfc3986#section-2.3
func isInValidCodeVerifier(codeVerifier string) bool {
	codeLen := len(codeVerifier)
	if codeLen < 43 || codeLen > 128 {
		return true
	}

	// unreserved characters
	for _, c := range codeVerifier {
		valid := (('A' <= c && c <= 'Z') ||
			('a' <= c && c <= 'z') ||
			('0' <= c && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~')
		if !valid {
			return true
		}
	}

	return false
}

func invalidCodeBinding(form tokenFormData, session OAuthSession) (invalid bool, description string) {
	if form.redirectURI != session.RedirectURI {
		return true, "redirect_uri does not match the one used in the authorization request"
	}
	if form.clientID != session.ClientID {
		return true, "client_id does not match the one used in the authorization request"
	}

	// 验证 code_verifier，流程见 https://datatracker.ietf.org/doc/html/rfc7636#section-4.6
	// BASE64(SHA256(code_verifier)) -> code_challenge
	shaCode := sha256.Sum256([]byte(form.codeVerifier))
	codeChallenge := base64.StdEncoding.EncodeToString(shaCode[:])
	if codeChallenge != session.CodeChallenge {
		return true, "code_verifier does not match the one used in the authorization request"
	}
	return false, ""
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (h *OAuthHandler) responseExchangeErr(w http.ResponseWriter, errCode OAExchangeTokenErr, description string) {
	httpjson.ResponseJSON(w, http.StatusBadRequest, tokenErrorResponse{
		Error:            string(errCode),
		ErrorDescription: description,
	}, h.log.Error())
}
