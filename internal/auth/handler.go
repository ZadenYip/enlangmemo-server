package auth

import (
	"log/slog"
	"net/http"

	"github.com/alexedwards/argon2id"
)

type AuthHandler struct {
	users    UserStore
	sessions SessionStore
	errLog   *slog.Logger
}

func NewAuthHandler(users UserStore, sessions SessionStore, errLog *slog.Logger) *AuthHandler {
	return &AuthHandler{
		users:    users,
		sessions: sessions,
		errLog:   errLog,
	}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
}

// 参数设置参考
// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#introduction
var argon2Params = argon2id.Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}
