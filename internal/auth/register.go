package auth

import (
	"errors"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	valid "github.com/zadenyip/enlangmemo-server/internal/validation"
)

type RegisterRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	UserID string `json:"userId"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	// TODO 加入限制请求频率的中间件，防止暴力破解密码
	var reg RegisterRequest

	if err := httpjson.DecodeJSONBody(w, r, &reg); err != nil {
		httpjson.HandleJSONDecodeError(w, err)
		return
	}

	if err := valid.ValidMaxChars("name", reg.Name, 16); err != nil {
		valid.HandleValidationError(w, err)
		return
	}

	if err := valid.ValidMaxChars("password", reg.Password, 32); err != nil {
		valid.HandleValidationError(w, err)
		return
	}

	passwdHash, err := argon2id.CreateHash(reg.Password, &argon2Params)
	if err != nil {
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to hash password")
		return
	}

	userID, err := h.users.CreateUser(r.Context(), reg.Name, passwdHash)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			httpjson.ResponseError(w, aip.StatusAlreadyExists, "User already exists")
			return
		}

		httpjson.ResponseError(w, aip.StatusInternal, "Failed to create user")
		return
	}

	httpjson.ResponseJSON(w, http.StatusCreated, RegisterResponse{UserID: userID})
}
