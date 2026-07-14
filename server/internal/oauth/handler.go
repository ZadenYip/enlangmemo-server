package oauth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/server/session/sso"
)

type OAuthHandler struct {
	oaStore  OAStorer
	ssoStore sso.SSOStore
	log      logging.Logger
}

func NewOAuthHandler(oaStore OAStorer, ssoStore sso.SSOStore, logger logging.Logger) *OAuthHandler {
	return &OAuthHandler{
		oaStore:  oaStore,
		ssoStore: ssoStore,
		log:      logger,
	}
}

func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /oauth/authorize", h.authorize)
}
