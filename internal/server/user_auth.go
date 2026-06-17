package server

import (
	"errors"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zadenyip/enlangmemo-server/internal/httpjson"
	"github.com/zadenyip/enlangmemo-server/internal/validation"
)

type RegisterRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type RegisterResponse struct {
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
		const hStatus = http.StatusInternalServerError
		httpjson.ResponseError(w, hStatus, "INTERNAL", "Failed to hash password")
		return
	}

	const insertUser = `
	INSERT INTO users (name, password_hash) VALUES ($1, $2)
	`
	tag, err := srv.dbPool.Exec(r.Context(), insertUser, reg.Name, passwdHash)

	if err != nil {
		var pgErr *pgconn.PgError
		// unique_violation 23505：see https://www.postgresql.org/docs/current/errcodes-appendix.html
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			const hStatus = http.StatusConflict
			httpjson.ResponseError(w, hStatus, "CONFLICT", "User already exists")
			return
		} else {
			const hStatus = http.StatusInternalServerError
			httpjson.ResponseError(w, hStatus, "INTERNAL", "Failed to create user")
			return
		}
	}

	tagRowsAffected := tag.RowsAffected()
	if tagRowsAffected != 1 {
		const hStatus = http.StatusInternalServerError
		httpjson.ResponseError(w, hStatus, "INTERNAL", "Failed to create user")
		return
	}

	httpjson.ResponseJSON(w, http.StatusCreated, RegisterResponse{})
}

type LoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (srv *Server) Login(w http.ResponseWriter, r *http.Request) {
	// TODO
}
