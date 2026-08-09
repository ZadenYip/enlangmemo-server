package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Handshake(
	ctx context.Context,
	r *connect.Request[syncv1.HandshakeRequest],
) (*connect.Response[syncv1.HandshakeResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("handshake not implemented"))
}
