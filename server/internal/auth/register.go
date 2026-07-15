package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	valid "github.com/zadenyip/enlangmemo-server/internal/validation"
)

type RegisterRequest struct {
	valid.Validator `json:"-"`
	LoginID         string `json:"loginId"`
	Nickname        string `json:"nickname"`
	Password        string `json:"password"`
}

type RegisterResponse struct {
	UserID string `json:"userId"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	// TODO 加入限制请求频率的中间件，防止暴力破解密码
	var reg RegisterRequest

	if err := httpjson.DecodeJSONBody(w, r, &reg); err != nil {
		httpjson.HandleJSONDecodeError(w, err, h.log.Error())
		return
	}
	reg.LoginID = strings.ToLower(reg.LoginID)
	h.log.InfoCtx(r.Context(), "register request received", "loginId", reg.LoginID, "nickname", reg.Nickname)

	reg.CheckField(valid.NotBlank(reg.LoginID), "loginId", "loginId must not be blank")
	reg.CheckField(valid.MaxChars(reg.LoginID, 16), "loginId", "loginId must not be longer than 16 characters")
	reg.CheckField(valid.ASCIIAlnum(reg.LoginID), "loginId", "loginId must contain only English letters and digits")
	reg.CheckField(valid.NotBlank(reg.Nickname), "nickname", "nickname must not be blank")
	reg.CheckField(valid.MaxChars(reg.Nickname, 16), "nickname", "nickname must not be longer than 16 characters")
	reg.CheckField(valid.MinChars(reg.Password, 8), "password", "password must be at least 8 characters")
	reg.CheckField(valid.MaxChars(reg.Password, 16), "password", "password must not be longer than 16 characters")
	if !reg.Valid() {
		reg.FailMsg = "invalid register request"
		h.log.InfoCtx(r.Context(), "invalid register request", "loginId", reg.LoginID, "nickname", reg.Nickname)
		valid.HandleValidationError(w, &reg.Validator, h.log.Error())
		return
	}

	passwdHash, err := argon2id.CreateHash(reg.Password, &argon2Params)
	if err != nil {
		h.log.ErrorCtx(r.Context(), "failed to hash password", "error", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Failed to hash password", h.log.Error())
		return
	}

	userID, err := h.users.CreateUser(r.Context(), reg.LoginID, reg.Nickname, passwdHash)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			h.log.InfoCtx(r.Context(), "user already exists", "loginId", reg.LoginID)
			httpjson.ResponseStatusError(w, aip.StatusAlreadyExists, "User already exists", h.log.Error())
			return
		}

		h.log.ErrorCtx(r.Context(), "failed to create user", "error", err)
		httpjson.ResponseStatusError(w, aip.StatusInternal, "Failed to create user", h.log.Error())
		return
	}

	h.log.InfoCtx(r.Context(), "user registered successfully", "userId", userID, "loginId", reg.LoginID, "nickname", reg.Nickname)
	httpjson.ResponseJSON(w, http.StatusCreated, RegisterResponse{UserID: userID}, h.log.Error())
}
