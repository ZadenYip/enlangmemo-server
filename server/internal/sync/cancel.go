package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) CancelSync(ctx context.Context,
	req *connect.Request[syncv1.CancelSyncRequest],
) (*connect.Response[syncv1.CancelSyncResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("cancel sync not implemented"))
}
