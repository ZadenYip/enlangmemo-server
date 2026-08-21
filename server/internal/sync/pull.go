package sync

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/zadenyip/enlangmemo-server/internal/sync/collector"
	ss "github.com/zadenyip/enlangmemo-server/internal/sync/session"
	syncv1 "github.com/zadenyip/enlangmemo-sync-api/packages/go/gen/enlangmemo/sync/v1"
)

func (h *SyncHandler) Pull(ctx context.Context,
	req *connect.Request[syncv1.PullRequest],
) (*connect.Response[syncv1.PullResponse], error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		// 不应该出现这个状况，因为 AuthInterceptor 已经放入 userID 进 context 了
		h.logger.ErrorCtx(ctx, "invalid userID after AuthInterceptor in Pull", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	claimResult, err := h.claimPullBatch(ctx, req.Msg, userID)
	if err != nil {
		h.logger.InfoCtx(ctx, "claimPullBatch failed", "userID", userID, "error", err)
		return nil, err
	}

	typeQueue, err := parseEntityQueue(claimResult.PullEntityQueue)
	if err != nil {
		h.logger.ErrorCtx(ctx, "failed to parse entity queue in Pull", "userID", userID, "pullEntityQueue", claimResult.PullEntityQueue, "err", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	c := collector.NewPullCollector()
	pullResult, err := h.pulStore.GetChangesSinceUSN(ctx, PullInfo{
		UserID:    userID,
		StartUSN:  claimResult.SyncCursorUSN,
		EndUSN:    claimResult.SrvSyncCursorUSNAtHandshake,
		typeQueue: typeQueue,
	}, c)
	if err != nil {
		h.logger.ErrorCtx(ctx, "GetChangesSinceUSN failed", "userID", userID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	queueStr := TypeQueueToString(pullResult.typeQueue)
	h.logger.InfoCtx(ctx, "pull result", "userID", userID, "queue", queueStr)

	// pull 完了所有实体类型
	if len(pullResult.typeQueue) == 0 {
		if err := h.sessionStore.MarkPullFinished(ctx, userID, req.Msg.GetSessionId()); err != nil {
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
		return connect.NewResponse(&syncv1.PullResponse{
			BatchSeq:    req.Msg.BatchSeq,
			Changes:     c.Changes(),
			BatchMaxUsn: c.MaxUSN(),
			LastBatch:   true,
		}), nil
	} else {
		// 还有其他数据没拉完，保存当前的进度
		if err := h.sessionStore.UpdatePullProgress(ctx, userID, req.Msg.GetSessionId(), queueStr, pullResult.SyncCursorUSN); err != nil {
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
		return connect.NewResponse(&syncv1.PullResponse{
			BatchSeq:    req.Msg.BatchSeq,
			Changes:     c.Changes(),
			BatchMaxUsn: c.MaxUSN(),
			LastBatch:   false,
		}), nil
	}
}

// claimPullBatch 将 store 的 ClaimPullBatch 方法的返回值转换为 connect.Error
func (h *SyncHandler) claimPullBatch(ctx context.Context, req *syncv1.PullRequest, userID int64) (ss.ClaimPullBatchResult, error) {
	result, err := h.sessionStore.ClaimPullBatch(ctx, userID, req.GetSessionId(), req.GetBatchSeq())
	if err != nil {
		h.logger.ErrorCtx(ctx, "ClaimPullBatch failed", "userID", userID, "sessionID", req.GetSessionId(), "batchSeq", req.GetBatchSeq(), "error", err)
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

// parseEntityQueue 将 typeQueue（entitiesTypeQueue）转为
// 如果没有 entityType，则返回 (0, nil)
// 如果有 entityType，则返回 (entityType, nil)
// 如果转换 type 为整数失败，则返回 (0, error)
func parseEntityQueue(typeQueue string) ([]int8, error) {
	if typeQueue == "" {
		return nil, fmt.Errorf("empty entities queue: %s", typeQueue)
	}

	entities := strings.Split(typeQueue, ",")

	// Split 只有空字符串和分割字符为空时，len 才会返回 0
	// if len(entities) == 0 {
	//	return nil, fmt.Errorf("invalid entities queue: %s", typeQueue)
	//}

	intTypeQueue := make([]int8, 0, 7)

	for _, value := range entities {
		entityType, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		intTypeQueue = append(intTypeQueue, int8(entityType))
	}

	return intTypeQueue, nil
}

func TypeQueueToString(typeQueue []int8) string {
	if len(typeQueue) == 0 {
		return ""
	}

	strs := make([]string, len(typeQueue))
	for i, t := range typeQueue {
		strs[i] = strconv.Itoa(int(t))
	}

	return strings.Join(strs, ",")
}
