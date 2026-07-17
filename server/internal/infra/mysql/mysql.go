package mysql

import (
	"context"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func NewClient(dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		panic(err)
	}
	return db
}
