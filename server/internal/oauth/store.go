package oauth

import (
	"context"
	"database/sql"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
)

type OAStorer interface {
	// 获取 OAuth 客户端信息
	GetClientInfo(ctx context.Context, clientID string) (OAClientInfo, error)
	// 生成并存储授权码和会话信息
	GenCodeStoreSession(ctx context.Context, authoInfo AuthorizationInfo) (string, error)
	ConsumeCodeSession(ctx context.Context, authCode string) (OAuthSession, error)
	GenAccessToken(ctx context.Context, clientID string, userID int64) (string, error)
	// 根据访问令牌获取用户 ID
	GetUserIDByAccessToken(ctx context.Context, accessToken string) (int64, error)
	GetTokenInfoByAccessToken(ctx context.Context, accessToken string) (TokenInfo, error)

	// RevokeAccessToken 撤销访问令牌
	//
	// 返回的 error 解释：
	// clientID 对不上任意客户端，则返回 errOAClientNotFound
	// 访问令牌不存在时返回 ErrAccessTokenNotFound
	// 令牌绑定的 clientID 与传入的 clientID 不匹配时返回 ErrAccessTokenClientMismatch
	RevokeAccessToken(ctx context.Context, accessToken, clientID string) error
}

type OAStore struct {
	db     *sql.DB
	rdb    *redis.Client
	logger logging.Logger
}

func NewOAStore(db *sql.DB, rdb *redis.Client, logger logging.Logger) *OAStore {
	return &OAStore{
		db:     db,
		rdb:    rdb,
		logger: logger,
	}
}
