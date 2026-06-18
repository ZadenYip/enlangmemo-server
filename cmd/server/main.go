package main

import (
	"log"
	"net/http"

	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/pg"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
	"github.com/zadenyip/enlangmemo-server/internal/server"
	"github.com/zadenyip/enlangmemo-server/internal/server/middleware"
)

func main() {
	config := config.Load()
	dbPool := pg.NewClient(config.DatabaseURL)
	defer dbPool.Close()

	rdb := redisclient.NewClient(config.RedisURL)

	server := server.New(dbPool, rdb)
	handler := server.Routes()

	handler = middleware.Logging(handler)
	handler = middleware.PanicRecovery(handler)
	log.Fatal(http.ListenAndServe(":8080", handler))
}
