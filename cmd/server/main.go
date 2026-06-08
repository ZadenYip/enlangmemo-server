package main
import (
	"github.com/zadenyip/enlangmemo-server/internal/config"
	"github.com/zadenyip/enlangmemo-server/internal/infra/pg"
	"github.com/zadenyip/enlangmemo-server/internal/infra/redisclient"
)

func main() {
	config := config.Load()
	db := pg.NewClient(config.DatabaseURL)
	rdb := redisclient.NewClient(config.RedisURL)

}
