package server

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userStore interface {
	CreateUser(ctx context.Context, name string, passwordHash string) (pgtype.UUID, error)
}

var errUserAlreadyExists = errors.New("user already exists")

type pgUserStore struct {
	dbPool *pgxpool.Pool
}

func (store *pgUserStore) CreateUser(ctx context.Context, name string, passwordHash string) (pgtype.UUID, error) {
	const insertUser = `
		INSERT INTO users (name, password_hash) VALUES ($1, $2)
		RETURNING id
	`

	var userID pgtype.UUID
	err := store.dbPool.QueryRow(ctx, insertUser, name, passwordHash).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		// unique_violation 23505: see https://www.postgresql.org/docs/current/errcodes-appendix.html
		// 默认隔离级别 read committed 配合 unique constraint 防止 read skew 导致的重复用户创建
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return pgtype.UUID{}, errUserAlreadyExists
		}

		return pgtype.UUID{}, err
	}

	return userID, nil
}
