package auth

import (
	"errors"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type LogoutResponse struct {
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sso.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			// 如果没有 cookie，仍然返回 200 OK，并清除 cookie
			expiredCookie := sso.GenerateExpiredCookie()
			http.SetCookie(w, &expiredCookie)
			httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{}, h.log.Error())
			return
		}

		h.log.ErrorCtx(r.Context(), "failed to read session cookie", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Failed to read session cookie", h.log.Error())
		return
	}

	// cookie 值为空的情况也应该返回 200 OK，并清除客户端 cookie
	if cookie.Value == "" {
		expiredCookie := sso.GenerateExpiredCookie()
		http.SetCookie(w, &expiredCookie)
		httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{}, h.log.Error())
		return
	}

	if err := h.sso.Logout(r.Context(), cookie.Value); err != nil {
		h.log.ErrorCtx(r.Context(), "failed to delete session", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Failed to delete session", h.log.Error())
		return
	}

	expiredCookie := sso.GenerateExpiredCookie()
	http.SetCookie(w, &expiredCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{}, h.log.Error())
}
