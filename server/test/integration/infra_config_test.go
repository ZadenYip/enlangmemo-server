package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/mysql"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
)

func TestInfraConfigClients(t *testing.T) {
	resetEnv(t)

	cfg := config.Load()
	require.Equal(t, env.dbURL, cfg.DatabaseURL)
	require.Equal(t, env.rdsURL, cfg.RedisURL)

	db := mysql.NewClient(cfg.DatabaseURL)
	defer db.Close()
	require.NoError(t, db.PingContext(t.Context()))

	rdb := redisclient.NewClient(cfg.RedisURL)
	defer func() {
		require.NoError(t, rdb.Close())
	}()
	require.NoError(t, rdb.Ping(t.Context()).Err())
}

func TestInfraConfigClientInvalidURLs(t *testing.T) {
	resetEnv(t)

	require.Panics(t, func() {
		mysql.NewClient("://invalid")
	})

	require.Panics(t, func() {
		redisclient.NewClient("://invalid")
	})
}
