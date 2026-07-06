package auth

import (
	"errors"
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
		httpjson.HandleJSONDecodeError(w, err, h.errLog)
		return
	}

	req.CheckField(valid.MaxChars(req.Name, 16), "name", "name must not be longer than 16 characters")
	req.CheckField(valid.MaxChars(req.Password, 32), "password", "password must not be longer than 32 characters")
	if !req.Valid() {
		req.FailMsg = "Invalid login request"
		valid.HandleValidationError(w, &req.Validator, h.errLog)
		return
	}

	userID, actualHash, err := h.users.GetPasswordHash(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpjson.ResponseError(w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusNotFound).
					WithMessage("User not found"),
				h.errLog)
			return
		} else {
			h.errLog.Error("failed to get password hash",
				"user", req.Name,
				"err", err,
			)
			httpjson.ResponseError(w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusInternal).
					WithMessage("Failed to get password hash"),
				h.errLog)
			return
		}
	}

	match, err := argon2id.ComparePasswordAndHash(req.Password, actualHash)
	if err != nil {
		h.errLog.Error("failed to compare password and hash",
			"user", req.Name,
			"err", err,
		)
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to compare password and hash"),
			h.errLog)
		return
	}

	if !match {
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusUnauthenticated).
				WithMessage("Invalid password"),
			h.errLog)
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), userID)
	if err != nil {
		h.errLog.Error("failed to create session",
			"user", req.Name,
			"err", err,
		)
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to create session"),
			h.errLog)
		return
	}

	ssoCookie := sso.GenerateCookie(sessionID)
	http.SetCookie(w, &ssoCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LoginResponse{}, h.errLog)
}
