package oauth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
)

// https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
type OAErr string

const (
	authorInvalidRequest OAErr = "invalid_request"
	// authorUnauthorizedClient      OAErr = "unauthorized_client"
	// authorAccessDenied            OAErr = "access_denied"
	// authorUnsupportedResponseType OAErr = "unsupported_response_type"
	// authorInvalidScope            OAErr = "invalid_scope"
	authorServerError OAErr = "server_error"
	// authorTemporarilyUnavailable  OAErr = "temporarily_unavailable"
)

// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
const (
	exInvalidRequest     OAErr = authorInvalidRequest
	exInvalidClient      OAErr = "invalid_client"
	exInvalidGrant       OAErr = "invalid_grant"
	exUnauthorizedClient OAErr = "unauthorized_client"
	exUnsupportedGrant   OAErr = "unsupported_grant_type"
	exInvalidScope       OAErr = "invalid_scope"
)

const (
	revokeUnsupportedTokenType OAErr = "unsupported_token_type"
)

// responseBadReqErr 用来响应客户端请求错误，遵循 RFC 6749 Section 5.2 的规范
// 响应 JSON 格式
func (h *OAuthHandler) responseBadReqErr(w http.ResponseWriter, errCode OAErr, description string) {
	httpjson.ResponseJSON(w, http.StatusBadRequest, tokenErrorResponse{
		Error:            string(errCode),
		ErrorDescription: description,
	}, h.log.Error())
}

// responseSrvInternalErr 用来响应服务器内部错误，遵循 RFC 6749 Section 5.2 的错误代码
// 不过这里是响应 JSON 格式，对于要求返回 JSON 格式的响应，RFC 6749 并没说是否要遵循 5.2 错误代码
func (h *OAuthHandler) responseSrvInternalErr(w http.ResponseWriter) {
	httpjson.ResponseJSON(w, http.StatusInternalServerError, tokenErrorResponse{
		Error:            string(authorServerError),
		ErrorDescription: "Internal server error",
	}, h.log.Error())
}
