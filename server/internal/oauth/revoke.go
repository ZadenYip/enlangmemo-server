package oauth

import (
	"errors"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

// https://datatracker.ietf.org/doc/html/rfc7009#section-2.1
// 这里没 token_type_hint 的处理，因为只有一种 token，也就是 access_token
type revokeForm struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id"`
}

// revoke 处理 OAuth2.0 Token 撤销请求
// https://datatracker.ietf.org/doc/html/rfc7009#section-2.1
// 这里没有 refresh_token 的处理，因为目前整个系统没用 refresh_token
func (h *OAuthHandler) revoke(w http.ResponseWriter, r *http.Request) {
	rForm, ok := h.extractRevokeForm(w, r)
	if !ok {
		return
	}

	if invalid, describe := isInvalidRevokeForm(rForm); invalid {
		h.responseBadReqErr(w, authorInvalidRequest, describe)
		return
	}

	err := h.oaStore.RevokeAccessToken(r.Context(), rForm.Token, rForm.ClientID)
	// 根据协议要先验证 clientID 是否是未知客户端，再验证访问令牌是否存在以及 clientID 是否匹配
	switch {
	case errors.Is(err, errOAClientNotFound):
		const msg = "unknown client_id when revoking access token"
		h.log.InfoCtx(r.Context(), msg, "client_id", rForm.ClientID)
		h.responseBadReqErr(w, exInvalidClient, "Invalid client")
		return

	case errors.Is(err, ErrAccessTokenNotFound):
		const msg = "access token not found when revoking"
		h.log.InfoCtx(r.Context(), msg, "access_token", logging.MaskSecret(rForm.Token), "client_id", rForm.ClientID)
		// 协议要求访问令牌不存在时也返回 200 OK
		// 这里直接走到代码的最后，返回 200 OK

	case errors.Is(err, errAccessTokenClientMismatch):
		const msg = "access token client mismatch when revoking"
		h.log.InfoCtx(r.Context(), msg, "access_token", logging.MaskSecret(rForm.Token), "client_id", rForm.ClientID)
		// 协议要求访问令牌 clientID 不匹配时也返回 200 OK
		// 这里不 return，而是走到末尾返回 200 OK

	case err != nil:
		h.log.ErrorCtx(r.Context(), "failed to revoke access token", "err", err)
		// 协议似乎没规定服务器内部错误返回格式，暂时返回 JSON
		h.responseSrvInternalErr(w)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *OAuthHandler) extractRevokeForm(w http.ResponseWriter, r *http.Request) (revokeForm, bool) {
	err := r.ParseForm()
	if err != nil {
		h.log.InfoCtx(r.Context(), "failed to parse form when extracting revoke form: %v", err)
		h.responseBadReqErr(w, authorInvalidRequest, "failed to parse form")
		return revokeForm{}, false
	}

	pForm := r.PostForm
	if pForm == nil {
		return revokeForm{}, false
	}

	revokeForm := revokeForm{
		Token:    pForm.Get("token"),
		ClientID: pForm.Get("client_id"),
	}

	return revokeForm, true
}

// isInvalidRevokeForm 验证请求参数是否有效（格式），如果无效则返回 true 和错误描述
func isInvalidRevokeForm(rForm revokeForm) (invalid bool, description string) {
	if rForm.ClientID == "" {
		return true, "client_id is required"
	}
	if rForm.Token == "" {
		return true, "token is required"
	}

	return false, ""
}
