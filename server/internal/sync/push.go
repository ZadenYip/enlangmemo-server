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
// claimPushBatch 会校验 Push session 状态和 batch seq，并领取当前 batch 的 assigned_usn。
func (h *SyncHandler) claimPushBatch(ctx context.Context, req *syncv1.PushRequest, userID string) (ClaimPushBatchResult, error) {
	result, err := h.sessionStore.ClaimPushBatch(
		ctx,
		userID,
		req.GetSessionId(),
		int64(req.GetBatchSeq()),
	)
	if err != nil {
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}

	switch result.LuaResult {
	case ClaimPushBatchLuaOK:
		return result, nil
	case ClaimPushBatchLuaSessionNotFound:
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session not found"))
	case ClaimPushBatchLuaSessionIDMismatch:
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session id mismatch"))
	case ClaimPushBatchLuaBatchSeqMismatch:
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync batch seq mismatch"))
	case ClaimPushBatchLuaStateMismatch:
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session state mismatch"))
	default:
		return ClaimPushBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}
}
