package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userStore interface {
	CreateUser(ctx context.Context, name string, passwordHash string) error
}

var errUserAlreadyExists = errors.New("user already exists")

type pgUserStore struct {
	dbPool *pgxpool.Pool
}

func (store *pgUserStore) CreateUser(ctx context.Context, name string, passwordHash string) error {
	const insertUser = `
		INSERT INTO users (name, password_hash) VALUES ($1, $2)
	`

	tag, err := store.dbPool.Exec(ctx, insertUser, name, passwordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		// unique_violation 23505: see https://www.postgresql.org/docs/current/errcodes-appendix.html
		// 默认隔离级别 read committed 配合 unique constraint 防止 read skew 导致的重复用户创建
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errUserAlreadyExists
		}

		return err
	}

	if rowsAffected := tag.RowsAffected(); rowsAffected != 1 {
		return fmt.Errorf("create user: expected 1 row affected, got %d", rowsAffected)
	}

	return nil
}
