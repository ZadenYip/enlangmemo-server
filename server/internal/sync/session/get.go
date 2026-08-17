package session

import (
	"context"
	"fmt"
)

// GetSession 尝试获取 SyncSession，如果不存在则返回错误。
// 一般这个用来打印日志用，而不是正式的业务逻辑
func (s *SessionStore) GetSession(ctx context.Context, userID int64) (SyncSession, error) {
	cmd := s.rdb.HGetAll(ctx, rdbSessionKey(userID))
	fields, err := cmd.Result()
	if err != nil {
		s.logger.ErrorCtx(ctx, "failed to get sync session", "userID", userID, "error", err)
		return SyncSession{}, err
	}
	if len(fields) == 0 {
		return SyncSession{}, fmt.Errorf("sync session not found for userID %d", userID)
	}

	var session SyncSession
	if err := cmd.Scan(&session); err != nil {
		s.logger.ErrorCtx(ctx, "failed to scan sync session", "userID", userID, "fields", fields, "error", err)
		return SyncSession{}, err
	}

	return session, nil
}
