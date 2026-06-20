package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/zadenyip/enlangmemo-server/internal/aip"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
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

	passwdHash, err := argon2id.CreateHash(reg.Password, &argon2Params)
	if err != nil {
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to hash password")
		return
	}

	userID, err := srv.usersStore.CreateUser(r.Context(), reg.Name, passwdHash)
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

type LoginResponse struct {
}

func (srv *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := httpjson.DecodeJSONBody(w, r, &req); err != nil {
		httpjson.HandleJSONDecodeError(w, err)
		return
	}

	userID, actualHash, err := srv.usersStore.GetPasswordHash(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
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

	sessionID, err := srv.ssoStore.Create(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to create session for user %s: %v", req.Name, err)
		httpjson.ResponseError(w, aip.StatusInternal, "Failed to create session")
		return
	}

	ssoCookie := sso.GenerateCookie(sessionID)
	http.SetCookie(w, &ssoCookie)
	httpjson.ResponseJSON(w, http.StatusOK, LoginResponse{})
}
