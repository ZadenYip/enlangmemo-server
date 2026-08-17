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

// claimPullBatch 将 store 的 ClaimPullBatch 方法的返回值转换为 connect.Error
func (h *SyncHandler) claimPullBatch(ctx context.Context, req *syncv1.PullRequest, userID int64) (ss.ClaimPullBatchResult, error) {
	result, err := h.sessionStore.ClaimPullBatch(ctx, userID, req.GetSessionId(), req.GetBatchSeq())
	if err != nil {
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}

	switch result.LuaResult {
	case ss.ClaimPullBatchLuaOK:
		return result, nil
	case ss.ClaimPullBatchLuaSessionNotFound:
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session not found"))
	case ss.ClaimPullBatchLuaSessionIDMismatch:
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session id mismatch"))
	case ss.ClaimPullBatchLuaBatchSeqMismatch:
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync batch seq mismatch"))
	case ss.ClaimPullBatchLuaStateMismatch:
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session state mismatch"))
	default:
		return ss.ClaimPullBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}
}
