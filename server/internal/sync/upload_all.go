package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) UploadAllPrepare(ctx context.Context,
	req *connect.Request[syncv1.UploadAllPrepareRequest],
) (*connect.Response[syncv1.UploadAllPrepareResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("upload all prepare not implemented"))
}

func (h *SyncHandler) UploadAllPush(ctx context.Context,
	req *connect.Request[syncv1.UploadAllPushRequest],
) (*connect.Response[syncv1.UploadAllPushResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("upload all push not implemented"))
}
