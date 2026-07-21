package server

import (
	"database/sql"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/apps/enlangmemo"
	"github.com/zadenyip/enlangmemo-server/internal/auth"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-server/internal/server/middleware"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type Server struct {
	log               logging.Logger
	authHandler       *auth.AuthHandler
	oauthHandler      *oauth.OAuthHandler
	enlangmemoHandler *enlangmemo.Handler
}

type StoreDeps struct {
	DB  *sql.DB
	Rdb *redis.Client
}

// 注册路由标签函数
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

func New(storeDeps StoreDeps, logger logging.Logger) *Server {

	userStore := auth.NewMySQLUserStore(storeDeps.DB)
	ssoStore := &sso.RedisSSOStore{Rdb: storeDeps.Rdb}

	// Auth handler
	authHandler := auth.NewAuthHandler(userStore, ssoStore, logger)

	// OAuth handler
	oaStore := oauth.NewOAStore(storeDeps.DB, storeDeps.Rdb, logger)
	oauthHandler := oauth.NewOAuthHandler(oaStore, ssoStore, logger)

	enlangmemoHandler := enlangmemo.NewHandler(oaStore, userStore, logger)

	return &Server{
		log:               logger,
		authHandler:       authHandler,
		oauthHandler:      oauthHandler,
		enlangmemoHandler: enlangmemoHandler,
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

	// 注册 EnLangMemo 应用路由
	srv.enlangmemoHandler.RegisterRoutes(mux)
	return mux
}

func (srv *Server) GetHandler() http.Handler {
	handler := srv.routes()
	handler = middleware.Logging(handler, srv.log)
	handler = middleware.PanicRecovery(handler, srv.log)
	handler = middleware.Trace(handler, srv.log)

	return handler
}
