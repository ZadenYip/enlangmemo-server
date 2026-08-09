package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) FinishSync(ctx context.Context,
	req *connect.Request[syncv1.FinishSyncRequest],
) (*connect.Response[syncv1.FinishSyncResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("finish sync not implemented"))
}
