package main

import (
	"net/http"
	"os"

	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/pg"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

func main() {
	logger := server.NewServerLog()
	logger.Info().Info("starting server")

	config := config.Load()

	dbPool := pg.NewClient(config.DatabaseURL)
	logger.Info().Info("connected to postgres")
	defer dbPool.Close()

	rdb := redisclient.NewClient(config.RedisURL)
	logger.Info().Info("connected to redis")

	storeDeps := server.StoreDeps{
		PGPool: dbPool,
		Rdb:    rdb,
	}

	server := server.New(storeDeps, logger)
	handler := server.GetHandler()

	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Error().Error("server stopped", "err", err)
		os.Exit(1)
	}
}
