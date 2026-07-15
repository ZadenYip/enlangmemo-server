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
	cookie, err := r.Cookie(sso.SSOCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			// 如果没有 cookie，仍然返回 200 OK，并清除 cookie
			expiredCookie := sso.GenerateExpiredCookie(sso.SSOCookieName)
			h.log.InfoCtx(r.Context(), "no session cookie found, returning 200 OK and clearing cookie")
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
		expiredCookie := sso.GenerateExpiredCookie(sso.SSOCookieName)
		http.SetCookie(w, &expiredCookie)
		h.log.WarnCtx(r.Context(), "session cookie value is empty, returning 200 OK and clearing cookie")
		httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{}, h.log.Error())
		return
	}

	deletedCount, err := h.sso.Logout(r.Context(), cookie.Value)
	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to logout session", "err", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Failed to delete session", h.log.Error())
		return
	}
	if deletedCount == 0 {
		// 本身就没有这个 session
		h.log.WarnCtx(r.Context(), "session not found or already logged out", "sessionID", cookie.Value)
		httpjson.ResponseStatusError(w, aip.StatusNotFound, "Session not found or already logged out", h.log.Error())
		return
	}

	expiredCookie := sso.GenerateExpiredCookie(sso.SSOCookieName)
	http.SetCookie(w, &expiredCookie)
	h.log.InfoCtx(r.Context(), "session logged out successfully")
	httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{}, h.log.Error())
}
