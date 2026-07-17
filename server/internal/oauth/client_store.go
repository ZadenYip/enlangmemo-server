package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type OAClientInfo struct {
	ClientID    string `json:"clientID"`
	RedirectURI string `json:"redirectURI"`
}

const oaClientInfoCacheTTL = 10 * time.Minute

var errOAClientNotFound = errors.New("oauth client not found")

// GetClientInfo 会从 Redis 获取 OAuth 客户端信息，如果缓存不存在，则从数据库查询，并将结果缓存到 Redis
func (s *OAStore) GetClientInfo(ctx context.Context, clientID string) (OAClientInfo, error) {
	cacheKey := "oauth:client:" + clientID

	if clientInfo, ok := s.getCachedClientInfo(ctx, clientID, cacheKey); ok {
		return clientInfo, nil
	}

	// Redis 缓存没命中，从数据库查询 OAuth 客户端信息
	clientUUID, err := uuid.Parse(clientID)
	if err != nil {
		s.logger.InfoCtx(ctx, "invalid oauth client id", "clientID", clientID, "err", err)
		return OAClientInfo{}, errOAClientNotFound
	}

	var clientInfo OAClientInfo
	const query = `SELECT redirect_uri FROM oauth_clients WHERE id = ?`
	err = s.db.QueryRowContext(ctx, query, clientUUID[:]).Scan(&clientInfo.RedirectURI)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return OAClientInfo{}, errOAClientNotFound
	case err == nil:
		// 查询成功，并将结果缓存到 Redis
		clientInfo.ClientID = clientID
		s.cacheClientInfo(ctx, cacheKey, clientInfo)
	default:
		s.logger.ErrorCtx(ctx, "failed to query oauth client info from database", "clientID", clientID, "err", err)
		return OAClientInfo{}, err
	}

	return clientInfo, nil
}

// 获取 Redis 缓存中的 OAuth 客户端信息，如果缓存不存在或解析失败，则返回 false
func (s *OAStore) getCachedClientInfo(ctx context.Context, clientID string, cacheKey string) (OAClientInfo, bool) {
	result, err := s.rdb.GetEx(ctx, cacheKey, oaClientInfoCacheTTL).Result()
	if errors.Is(err, redis.Nil) {
		return OAClientInfo{}, false
	}
	if err != nil {
		s.logger.WarnCtx(ctx, "failed to get oauth client info from cache", "clientID", clientID, "err", err)
		return OAClientInfo{}, false
	}

	var clientInfo OAClientInfo
	if err := json.Unmarshal([]byte(result), &clientInfo); err == nil {
		return clientInfo, true
	}

	s.logger.WarnCtx(ctx, "failed to unmarshal cached oauth client info, deleting cache", "clientID", clientID, "err", err)
	if err := s.rdb.Del(ctx, cacheKey).Err(); err != nil {
		s.logger.WarnCtx(ctx, "failed to delete cached oauth client info after unmarshal failure", "clientID", clientID, "err", err)
	}

	return OAClientInfo{}, false
}

// 将 OAuth 客户端信息缓存到 Redis 中
func (s *OAStore) cacheClientInfo(ctx context.Context, cacheKey string, clientInfo OAClientInfo) {
	data, err := json.Marshal(clientInfo)
	if err != nil {
		s.logger.WarnCtx(ctx, "failed to marshal oauth client info for cache", "clientID", clientInfo.ClientID, "err", err)
		return
	}

	if err := s.rdb.Set(ctx, cacheKey, data, oaClientInfoCacheTTL).Err(); err != nil {
		s.logger.WarnCtx(ctx, "failed to set oauth client info in cache", "clientID", clientInfo.ClientID, "err", err)
	}
}
