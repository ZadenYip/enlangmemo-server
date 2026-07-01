package auth

import (
	"errors"
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type LogoutResponse struct {
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sso.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {

			// 如果没有 cookie，仍然返回 200 OK，并清除 cookie
			expiredCookie := sso.GenerateExpiredCookie()
			http.SetCookie(w, &expiredCookie)
			httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{})
			return
		}

		log.Printf("Failed to read session cookie: %v", err)
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to read session cookie")
		return
	}

	if err := h.sessions.Logout(r.Context(), cookie.Value); err != nil {
		log.Printf("Failed to delete session: %v", err)
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to delete session")
		return
	}

	expiredCookie := sso.GenerateExpiredCookie()
	http.SetCookie(w, &expiredCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LogoutResponse{})
}
