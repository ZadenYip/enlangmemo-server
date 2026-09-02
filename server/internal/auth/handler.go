package auth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type AuthHandler struct {
	users UserStore
	sso   SSOStore
	log   logging.Logger
}

func NewAuthHandler(users UserStore, sso SSOStore, log logging.Logger) *AuthHandler {
	return &AuthHandler{
		users: users,
		sso:   sso,
		log:   log,
	}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
}
