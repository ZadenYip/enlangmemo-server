package sync

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1/syncv1connect"
)

type SyncHandler struct {
	oaStore      oauth.OAStorer
	sessionStore SessionStorer
	hskStore     HandshakeStorer
	logger       logging.Logger
}

func NewSyncHandler(oaStore oauth.OAStorer, hskStore HandshakeStorer, sessionStore SessionStorer) *SyncHandler {
	return &SyncHandler{
		oaStore:      oaStore,
		hskStore:     hskStore,
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}
}

func (h *SyncHandler) RegisterRoutes(mux *http.ServeMux) {

	interceptors := connect.WithInterceptors(
		validate.NewInterceptor(),
		NewAuthInterceptor(h.oaStore),
	)

	path, httpHandler := syncv1connect.NewSyncServiceHandler(
		h,
		interceptors,
	)

	// path: /enlangmemo.sync.v1.SyncService/
	mux.Handle(path, httpHandler)
}
