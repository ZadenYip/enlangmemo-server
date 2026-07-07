package oauth

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

type OAStorer interface {
	// 获取 OAuth 客户端信息
	GetClientInfo(clientID string) (*OAClientInfo, error)
}

type OAStore struct {
	pgpool *pgxpool.Pool
	rdb    *redis.Client
}

func NewOAStore(storeDeps server.StoreDeps) *OAStore {
	return &OAStore{
		pgpool: storeDeps.PGPool,
		rdb:    storeDeps.Rdb,
	}
}
