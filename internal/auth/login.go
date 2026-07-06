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
	Name            string `json:"name"`
	Password        string `json:"password"`
	valid.Validator `json:"-"`
}

type LoginResponse struct {
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := httpjson.DecodeJSONBody(w, r, &req); err != nil {
		httpjson.HandleJSONDecodeError(w, err)
		return
	}

	req.CheckField(valid.MaxChars(req.Name, 16), "name", "name must not be longer than 16 characters")
	req.CheckField(valid.MaxChars(req.Password, 32), "password", "password must not be longer than 32 characters")
	if !req.Valid() {
		req.FailMsg = "Invalid login request"
		valid.HandleValidationError(w, &req.Validator)
		return
	}

	userID, actualHash, err := h.users.GetPasswordHash(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpjson.ResponseError(w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusNotFound).
					WithMessage("User not found"))
			return
		} else {
			log.Printf("Failed to get password hash for user %s: %v", req.Name, err)
			httpjson.ResponseError(w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusInternal).
					WithMessage("Failed to get password hash"))
			return
		}
	}

	match, err := argon2id.ComparePasswordAndHash(req.Password, actualHash)
	if err != nil {
		log.Printf("Failed to compare password and hash for user %s: %v", req.Name, err)
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to compare password and hash"))
		return
	}

	if !match {
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusUnauthenticated).
				WithMessage("Invalid password"))
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to create session for user %s: %v", req.Name, err)
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to create session"))
		return
	}

	ssoCookie := sso.GenerateCookie(sessionID)
	http.SetCookie(w, &ssoCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LoginResponse{})
}
