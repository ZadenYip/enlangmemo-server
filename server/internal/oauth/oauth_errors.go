package oauth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

// https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
type OAAuthorErr = string

const (
	authorInvalidRequest OAAuthorErr = "invalid_request"
	// authorUnauthorizedClient      OAAuthorErr = "unauthorized_client"
	// authorAccessDenied            OAAuthorErr = "access_denied"
	// authorUnsupportedResponseType OAAuthorErr = "unsupported_response_type"
	// authorInvalidScope            OAAuthorErr = "invalid_scope"
	authorServerError OAAuthorErr = "server_error"
	// authorTemporarilyUnavailable  OAAuthorErr = "temporarily_unavailable"
)

// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
type OAExchangeTokenErr = string

const (
	exInvalidRequest     OAExchangeTokenErr = string(authorInvalidRequest)
	exInvalidClient      OAExchangeTokenErr = "invalid_client"
	exInvalidGrant       OAExchangeTokenErr = "invalid_grant"
	exUnauthorizedClient OAExchangeTokenErr = "unauthorized_client"
	exUnsupportedGrant   OAExchangeTokenErr = "unsupported_grant_type"
	exInvalidScope       OAExchangeTokenErr = "invalid_scope"
)

// responseBadReqErr 用来响应客户端请求错误，遵循 RFC 6749 Section 5.2 的规范
func (h *OAuthHandler) responseBadReqErr(w http.ResponseWriter, errCode OAExchangeTokenErr, description string) {
	httpjson.ResponseJSON(w, http.StatusBadRequest, tokenErrorResponse{
		Error:            string(errCode),
		ErrorDescription: description,
	}, h.log.Error())
}
