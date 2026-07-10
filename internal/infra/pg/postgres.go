package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewClient(pgURL string) *pgxpool.Pool {
	dbpool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		panic(err)
	}
	if err := dbpool.Ping(context.Background()); err != nil {
		panic(err)
	}
	return dbpool
}
