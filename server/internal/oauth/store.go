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
	GenAccessToken(ctx context.Context, userID string) (string, error)
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
