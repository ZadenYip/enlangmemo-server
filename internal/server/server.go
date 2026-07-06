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

// 注册路由标签函数
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func New(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	userStore := auth.NewPGUserStore(dbPool)
	ssoStore := &sso.RedisSSOStore{Rdb: rdb}

	return &Server{
		authHandler: auth.NewAuthHandler(userStore, ssoStore),
	}
}

// register routes
func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// 注册注册和登录的路由
	srv.authHandler.RegisterRoutes(mux)

	return mux
}

func (srv *Server) GetHandler() http.Handler {
	handler := srv.routes()
	handler = middleware.Logging(handler)
	handler = middleware.PanicRecovery(handler)

	return handler
}
