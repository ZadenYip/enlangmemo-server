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
	valid.Validator `json:"-"`
	Name            string `json:"name"`
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

	reg.CheckField(valid.MaxChars(reg.Name, 16), "name", "name must not be longer than 16 characters")
	reg.CheckField(valid.MaxChars(reg.Password, 32), "password", "password must not be longer than 32 characters")
	if !reg.Valid() {
		reg.FailMsg = "Invalid register request"
		valid.HandleValidationError(w, &reg.Validator, h.log.Error())
		return
	}

	passwdHash, err := argon2id.CreateHash(reg.Password, &argon2Params)
	if err != nil {
		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to hash password"),
			h.log.Error())
		return
	}

	userID, err := h.users.CreateUser(r.Context(), reg.Name, passwdHash)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			httpjson.ResponseError(w,
				aip.NewErrResponse().
					WithCodeAndStatus(aip.StatusAlreadyExists).
					WithMessage("User already exists"),
				h.log.Error())
			return
		}

		httpjson.ResponseError(w,
			aip.NewErrResponse().
				WithCodeAndStatus(aip.StatusInternal).
				WithMessage("Failed to create user"),
			h.log.Error())
		return
	}

	httpjson.ResponseJSON(w, http.StatusCreated, RegisterResponse{UserID: userID}, h.log.Error())
}
