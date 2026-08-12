package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Pull(ctx context.Context,
	req *connect.Request[syncv1.PullRequest],
) (*connect.Response[syncv1.PullResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("pull not implemented"))
}
