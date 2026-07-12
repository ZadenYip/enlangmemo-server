package oauth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type OAuthHandler struct {
	oaStore OAStorer
	log     logging.Logger
}

func NewOAuthHandler(oaStore OAStorer, logger logging.Logger) *OAuthHandler {
	return &OAuthHandler{
		oaStore: oaStore,
		log:     logger,
	}
}

func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /oauth/authorize", h.authorize)
}
