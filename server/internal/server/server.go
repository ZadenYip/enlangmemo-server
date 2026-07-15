package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-server/internal/server/middleware"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type Server struct {
	log          logging.Logger
	authHandler  *auth.AuthHandler
	oauthHandler *oauth.OAuthHandler
}

type StoreDeps struct {
	PGPool *pgxpool.Pool
	Rdb    *redis.Client
}

// 注册路由标签函数
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func New(storeDeps StoreDeps, logger logging.Logger) *Server {

	userStore := auth.NewPGUserStore(storeDeps.PGPool)
	ssoStore := &sso.RedisSSOStore{Rdb: storeDeps.Rdb}

	// Auth handler
	authHandler := auth.NewAuthHandler(userStore, ssoStore, logger)

	// OAuth handler
	oaStore := oauth.NewOAStore(storeDeps.PGPool, storeDeps.Rdb, logger)
	oauthHandler := oauth.NewOAuthHandler(oaStore, ssoStore, logger)

	return &Server{
		log:          logger,
		authHandler:  authHandler,
		oauthHandler: oauthHandler,
	}
}

// register routes
func (srv *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", health)

	// 注册注册和登录的路由
	srv.authHandler.RegisterRoutes(mux)

	// 注册授权路由
	srv.oauthHandler.RegisterRoutes(mux)
	return mux
}

func (srv *Server) GetHandler() http.Handler {
	handler := srv.routes()
	handler = middleware.Logging(handler, srv.log)
	handler = middleware.PanicRecovery(handler, srv.log)
	handler = middleware.Trace(handler, srv.log)

	return handler
}
