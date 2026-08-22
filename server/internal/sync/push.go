package sync

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Push(ctx context.Context,
	req *connect.Request[syncv1.PushRequest],
) (*connect.Response[syncv1.PushResponse], error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		// 不应该出现这个状况，因为 AuthInterceptor 已经放入 userID 进 context 了
		h.logger.ErrorCtx(ctx, "invalid userID after AuthInterceptor in Push", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	result, err := h.claimPushBatch(ctx, req.Msg, userID)
	if err != nil {
		h.logger.InfoCtx(ctx, "claimPushBatch failed", "userID", userID, "error", err)
		return nil, err
	}

	// 应用 push changes
	assignedChanges, err := h.pshStore.ApplyPushChanges(ctx, userID, result.AssignedStartUSN, req.Msg.Changes)
	if err != nil {
		h.logger.InfoCtx(ctx, "ApplyPushChanges failed", "userID", userID, "error", err)
		if errors.Is(err, errInvalidPushChange) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	// 应用成功
	h.logger.InfoCtx(ctx, "ApplyPushChanges success", "userID", userID, "assignedStartUSN", result.AssignedStartUSN, "changeCount", len(req.Msg.Changes))
	// 如果是最后一个 batch，则标记 push 完成
	if req.Msg.LastBatch {
		if err := h.sessionStore.MarkPushFinished(ctx, userID, req.Msg.SessionId); err != nil {
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
	}
	return connect.NewResponse(&syncv1.PushResponse{
		BatchSeq: req.Msg.BatchSeq,
		Changes:  assignedChanges,
	}), nil
}

// claimPushBatch 校验 Push session 状态和 batch_seq，
// 并根据本 batch 的 changes 并推进 USN 到本 batch 的最后一条 change。
// 返回的 AssignedStartUSN 是本 batch 第一条 change 对应的 USN。
func (h *SyncHandler) claimPushBatch(ctx context.Context, req *syncv1.PushRequest, userID int64) (ss.ClaimPushBatchResult, error) {
	result, err := h.sessionStore.ClaimPushBatch(
		ctx,
		userID,
		req.GetSessionId(),
		req.GetBatchSeq(),
		len(req.GetChanges()),
	)
	if err != nil {
		h.logger.ErrorCtx(ctx, "ClaimPushBatch failed", "userID", userID, "sessionID", req.GetSessionId(), "batchSeq", req.GetBatchSeq(), "error", err)
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}

	switch result.LuaResult {
	case ss.ClaimPushBatchLuaOK:
		return result, nil
	case ss.ClaimPushBatchLuaSessionNotFound:
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session not found"))
	case ss.ClaimPushBatchLuaSessionIDMismatch:
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session id mismatch"))
	case ss.ClaimPushBatchLuaBatchSeqMismatch:
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync batch seq mismatch"))
	case ss.ClaimPushBatchLuaStateMismatch:
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("sync session state mismatch"))
	default:
		return ss.ClaimPushBatchResult{}, connect.NewError(connect.CodeInternal, nil)
	}
}
