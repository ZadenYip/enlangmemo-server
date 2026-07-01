package auth

import (
	"errors"
	"log"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
	valid "github.com/zadenyip/enlangmemo-server/internal/validation"
)

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type LoginResponse struct {
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := httpjson.DecodeJSONBody(w, r, &req); err != nil {
		httpjson.HandleJSONDecodeError(w, err)
		return
	}

	if err := valid.ValidMaxChars("name", req.Name, 16); err != nil {
		valid.HandleValidationError(w, err)
		return
	}

	if err := valid.ValidMaxChars("password", req.Password, 32); err != nil {
		valid.HandleValidationError(w, err)
		return
	}

	userID, actualHash, err := h.users.GetPasswordHash(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpjson.ResponseError(w, aip.StatusNotFound, "User not found")
			return
		} else {
			log.Printf("Failed to get password hash for user %s: %v", req.Name, err)
			httpjson.ResponseError(w, aip.StatusInternal, "Failed to get password hash")
			return
		}
	}

	match, err := argon2id.ComparePasswordAndHash(req.Password, actualHash)
	if err != nil {
		log.Printf("Failed to compare password and hash for user %s: %v", req.Name, err)
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to compare password and hash")
		return
	}

	if !match {
		httpjson.ResponseError(w, aip.StatusUnauthenticated, "Invalid password")
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to create session for user %s: %v", req.Name, err)
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to create session")
		return
	}

	ssoCookie := sso.GenerateCookie(sessionID)
	http.SetCookie(w, &ssoCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LoginResponse{})
}
