package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/server/middleware"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type Server struct {
	authHandler *auth.AuthHandler
}

func New(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	userStore := auth.NewPGUserStore(dbPool)
	ssoStore := &sso.RedisSSOStore{Rds: rdb}

	return &Server{
		authHandler: auth.NewAuthHandler(userStore, ssoStore),
	}
}

// register routes
func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// auth
	mux.HandleFunc("POST /v1/auth/register", srv.authHandler.Register)
	mux.HandleFunc("POST /v1/auth/login", srv.authHandler.Login)
	mux.HandleFunc("POST /v1/auth/logout", srv.authHandler.Logout)

	return mux
}

func (srv *Server) GetHandler() http.Handler {
	handler := srv.routes()
	handler = middleware.Logging(handler)
	handler = middleware.PanicRecovery(handler)

	return handler
}
