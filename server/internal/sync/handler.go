package sync

import (
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
	"github.com/zadenyip/enlangmemo-server/internal/server/shutdown"
	"github.com/zadenyip/enlangmemo-server/internal/sync/interceptor"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	"github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1/syncv1connect"
)

type SyncHandler struct {
	oaStore      oauth.OAStorer
	pshStore     PushChangeStorer
	pulStore     PullChangeStorer
	sessionStore ss.SessionStorer
	hskStore     HandshakeStorer
	logger       logging.Logger
}

func NewSyncHandler(oaStore oauth.OAStorer, pshStore PushChangeStorer, pulStore PullChangeStorer, hskStore HandshakeStorer, sessionStore ss.SessionStorer) *SyncHandler {
	return &SyncHandler{
		oaStore:      oaStore,
		pshStore:     pshStore,
		pulStore:     pulStore,
		hskStore:     hskStore,
		sessionStore: sessionStore,
		logger:       logging.NewServerLog(),
	}
}

func (h *SyncHandler) RegisterRoutes(mux *http.ServeMux) {

	interceptors := connect.WithInterceptors(
		validate.NewInterceptor(),
		interceptor.NewAuthInterceptor(h.oaStore),
	)

	path, httpHandler := syncv1connect.NewSyncServiceHandler(
		h,
		// 设置最大 Protobuf-Msg 大小为 512KB，防止恶意请求
		connect.WithReadMaxBytes(1024*512),
		interceptors,
	)

	// path: /enlangmemo.sync.v1.SyncService/
	mux.Handle(path, httpHandler)
}

func (h *SyncHandler) GracefulShutdown() error {
	store := h.pulStore.(shutdown.GracefulShutdowner)
	return store.GracefulShutdown()
}
