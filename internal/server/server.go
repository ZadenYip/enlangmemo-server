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
	log         Logger
	authHandler *auth.AuthHandler
}

type StoreDeps struct {
	PGPool *pgxpool.Pool
	Rdb    *redis.Client
}

// 注册路由标签函数
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func New(storeDeps StoreDeps, logger Logger) *Server {
	userStore := auth.NewPGUserStore(storeDeps.PGPool)
	ssoStore := &sso.RedisSSOStore{Rdb: storeDeps.Rdb}

	return &Server{
		log:         logger,
		authHandler: auth.NewAuthHandler(userStore, ssoStore, logger.Error()),
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
	handler = middleware.Logging(handler, srv.log.Info())
	handler = middleware.PanicRecovery(handler, srv.log.Error())

	return handler
}
