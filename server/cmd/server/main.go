package main

import (
	"net/http"
	"os"

	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/mysql"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/server"
)

func main() {
	logger := logging.NewServerLog()
	logger.Info().Info("starting server")

	config := config.Load()

	db := mysql.NewClient(config.DatabaseURL)
	logger.Info().Info("connected to mysql")
	defer db.Close()

	rdb := redisclient.NewClient(config.RedisURL)
	logger.Info().Info("connected to redis")

	storeDeps := server.StoreDeps{
		DB:  db,
		Rdb: rdb,
	}

	server := server.New(storeDeps, logger)
	handler := server.GetHandler()

	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Error().Error("server stopped", "err", err)
		os.Exit(1)
	}
}
