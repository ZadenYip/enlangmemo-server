package oauth

import (
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type OAuthHandler struct {
	oaStore OAStorer
	logger  logging.Logger
}

func NewOAuthHandler(oaStore OAStorer, logger logging.Logger) *OAuthHandler {
	return &OAuthHandler{
		oaStore: oaStore,
		logger:  logger,
	}
}

func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	// mux.HandleFunc("GET /oauth/authorize", h.authorize)
}
