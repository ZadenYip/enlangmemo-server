package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	dbPool *pgxpool.Pool
	rdb    *redis.Client
}

func New(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	return &Server{
		dbPool,
		rdb,
	}
}

// register routes
func (srv *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// mux.HandleFunc("POST /v1/users", srv.Register)
	// TODO mux.HandleFunc("POST /login", srv.)
	return mux
}
