package sync

import (
	"context"
	"time"

	_ "embed"

	"connectrpc.com/connect"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) FinishSync(ctx context.Context,
	req *connect.Request[syncv1.FinishSyncRequest],
) (*connect.Response[syncv1.FinishSyncResponse], error) {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		// 中间件已经处理了 token 验证，这里不应该出现空 userID
		h.logger.ErrorCtx(ctx, "invalid userID after AuthInterceptor in FinishSync", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	finTime := time.Now().UnixMilli()
	if err := h.sessionStore.FinishSync(ctx, userID, req.Msg.SessionId, finTime); err != nil {
		return nil, err
	}

	resp := &syncv1.FinishSyncResponse{
		ServerFinishedAt: finTime,
	}

	h.logger.InfoCtx(ctx, "sync finished", "userID", userID, "sessionID", req.Msg.SessionId, "finishTime", finTime)

	return connect.NewResponse(resp), nil
}
