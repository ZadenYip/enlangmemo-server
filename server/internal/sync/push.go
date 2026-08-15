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
	userID := ctx.Value("userID").(string)
	if userID == "" {
		// 不应该出现这个状况，因为 AuthInterceptor 已经放入 userID 进 context 了
		h.logger.ErrorCtx(ctx, "userID is empty after AuthInterceptor in Push")
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	result, err := h.claimPushBatch(ctx, req.Msg, userID)
	if err != nil {
		h.logger.InfoCtx(ctx, "claimPushBatch failed", "userID", userID, "error", err)
		return nil, err
	}

	// 应用 push changes
	err = h.pshStore.ApplyPushChanges(ctx, userID, result.AssignedUSN, req.Msg.Changes)
	if err != nil {
		h.logger.InfoCtx(ctx, "ApplyPushChanges failed", "userID", userID, "error", err)
		if errors.Is(err, errInvalidPushChange) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	// 应用成功
	h.logger.InfoCtx(ctx, "ApplyPushChanges success", "userID", userID, "assignedUSN", result.AssignedUSN)
	// 如果是最后一个 batch，则标记 push 完成
	if req.Msg.LastBatch {
		if err := h.sessionStore.MarkPushFinished(ctx, userID, req.Msg.SessionId); err != nil {
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
	}
	return connect.NewResponse(&syncv1.PushResponse{
		BatchSeq:    req.Msg.BatchSeq,
		AssignedUsn: result.AssignedUSN,
	}), nil
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
