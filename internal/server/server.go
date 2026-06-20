package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type Server struct {
	usersStore UserStore
	ssoStore   sso.SSOStore
}

func New(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	return &Server{
		usersStore: &pgUserStore{dbPool: dbPool},
		ssoStore:   &sso.RedisSSOStore{Rds: rdb},
	}
}

// register routes
func (srv *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/register", srv.Register)
	mux.HandleFunc("POST /v1/auth/login", srv.Login)
	return mux
}
