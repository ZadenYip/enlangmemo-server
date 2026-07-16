package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
)

// TestGetClientInfoCachesOAuthClient 测试 GetClientInfo 会将 OAuth 客户端信息缓存到 Redis 中
func TestGetClientInfoCachesOAuthClient(t *testing.T) {
	resetEnv(t)
	clientID := registerOAuthClient(t, testOAuthRedirectURI)
	store := oauth.NewOAStore(env.dbPool, env.rdsClient, logging.NewServerLog())
	cacheKey := "oauth:client:" + clientID

	// 确认缓存中不存在该客户端信息
	exists, err := env.rdsClient.Exists(t.Context(), cacheKey).Result()
	require.NoError(t, err)
	require.Zero(t, exists)

	// 调用 GetClientInfo 获取客户端信息
	clientInfo, err := store.GetClientInfo(t.Context(), clientID)
	require.NoError(t, err)
	require.Equal(t, oauth.OAClientInfo{
		ClientID:    clientID,
		RedirectURI: testOAuthRedirectURI,
	}, clientInfo)

	// 确认缓存中已经存在该客户端信息
	exists, err = env.rdsClient.Exists(t.Context(), cacheKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}
