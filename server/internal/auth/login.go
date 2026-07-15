package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
	valid "github.com/zadenyip/enlangmemo-server/internal/validation"
)

type LoginRequest struct {
	LoginID         string `json:"loginId"`
	Password        string `json:"password"`
	valid.Validator `json:"-"`
}

type LoginResponse struct {
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := httpjson.DecodeJSONBody(w, r, &req); err != nil {
		httpjson.HandleJSONDecodeError(w, err, h.log.Error())
		return
	}

	req.LoginID = strings.ToLower(req.LoginID)
	req.CheckField(valid.NotBlank(req.LoginID), "loginId", "loginId must not be blank")
	req.CheckField(valid.MaxChars(req.LoginID, 16), "loginId", "loginId must not be longer than 16 characters")
	req.CheckField(valid.ASCIIAlnum(req.LoginID), "loginId", "loginId must contain only English letters and digits")
	req.CheckField(valid.MinChars(req.Password, 8), "password", "password must be at least 8 characters")
	req.CheckField(valid.MaxChars(req.Password, 16), "password", "password must not be longer than 16 characters")
	if !req.Valid() {
		req.FailMsg = "invalid login request"
		h.log.InfoCtx(r.Context(), "invalid login request", "loginId", req.LoginID)
		valid.HandleValidationError(w, &req.Validator, h.log.Error())
		return
	}

	userID, actualHash, err := h.users.GetPasswordHash(r.Context(), req.LoginID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			h.log.InfoCtx(r.Context(), "user not found", "loginId", req.LoginID)
			httpjson.ResponseStatusError(w, aip.StatusUnauthenticated, "invalid login credentials", h.log.Error())
			return
		} else {
			h.log.ErrorCtx(r.Context(), "failed to get password hash",
				"loginId", req.LoginID,
				"err", err,
			)
			httpjson.ResponseStatusError(w, aip.StatusInternal, "failed to get password hash", h.log.Error())
			return
		}
	}

	match, err := argon2id.ComparePasswordAndHash(req.Password, actualHash)
	if err != nil {
		// 比较密码和哈希失败
		h.log.ErrorCtx(r.Context(), "failed to compare password and hash",
			"loginId", req.LoginID,
			"err", err,
		)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "failed to compare password and hash", h.log.Error())
		return
	}

	// 密码错误
	if !match {
		h.log.InfoCtx(r.Context(), "invalid password", "loginId", req.LoginID)
		httpjson.ResponseStatusError(w, aip.StatusUnauthenticated, "invalid login credentials", h.log.Error())
		return
	}

	sessionID, err := h.sso.Create(r.Context(), userID)
	if err != nil {
		// 创建 session 失败
		h.log.ErrorCtx(r.Context(), "failed to create session",
			"loginId", req.LoginID,
			"err", err,
		)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "failed to create session", h.log.Error())
		return
	}

	// 登录成功，返回 session cookie
	h.log.InfoCtx(r.Context(), "user logged in successfully", "userId", userID, "loginId", req.LoginID)
	ssoCookie := sso.GenerateCookie(sso.SSOCookieName, sessionID)
	http.SetCookie(w, &ssoCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LoginResponse{}, h.log.Error())
}
