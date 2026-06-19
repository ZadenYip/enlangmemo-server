package server

import (
	"errors"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

type RegisterRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	UserID string `json:"userId"`
}

func (srv *Server) Register(w http.ResponseWriter, r *http.Request) {
	// TODO 加入限制请求频率的中间件，防止暴力破解密码
	var reg RegisterRequest

	if err := httpjson.DecodeJSONBody(w, r, &reg); err != nil {
		httpjson.HandleJSONDecodeError(w, err)
		return
	}

	if err := validation.ValidMaxChars("name", reg.Name, 16); err != nil {
		handleValidationError(w, err)
		return
	}

	if err := validation.ValidMaxChars("password", reg.Password, 32); err != nil {
		handleValidationError(w, err)
		return
	}

	// 参数设置参考
	// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#introduction
	argon2Params := argon2id.Params{
		Memory:      19 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	passwdHash, err := argon2id.CreateHash(reg.Password, &argon2Params)
	if err != nil {
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to hash password")
		return
	}

	userID, err := srv.users.CreateUser(r.Context(), reg.Name, passwdHash)
	if err != nil {
		if errors.Is(err, errUserAlreadyExists) {
			httpjson.ResponseError(w, aip.StatusAlreadyExists, "User already exists")
			return
		}

		httpjson.ResponseError(w, aip.StatusInternal, "Failed to create user")
		return
	}

	httpjson.ResponseJSON(w, http.StatusCreated, RegisterResponse{UserID: userID.String()})
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (srv *Server) Login(w http.ResponseWriter, r *http.Request) {
	// TODO
}
