package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Push(ctx context.Context,
	req *connect.Request[syncv1.PushRequest],
) (*connect.Response[syncv1.PushResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("push not implemented"))
}
