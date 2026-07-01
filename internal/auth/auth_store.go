package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGUserStore struct {
	dbPool *pgxpool.Pool
}

func NewPGUserStore(dbPool *pgxpool.Pool) *PGUserStore {
	return &PGUserStore{dbPool: dbPool}
}

func (store *PGUserStore) CreateUser(ctx context.Context, name string, passwordHash string) (string, error) {
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
			return "", ErrUserAlreadyExists
		}

		return "", err
	}

	return userID.String(), nil
}

func (store *PGUserStore) GetPasswordHash(ctx context.Context, name string) (string, string, error) {
	const selectUser = `
		SELECT id, password_hash FROM users WHERE name = $1
	`

	var userID pgtype.UUID
	var storedPasswordHash string
	err := store.dbPool.QueryRow(ctx, selectUser, name).Scan(&userID, &storedPasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrUserNotFound
	}
	if err != nil {
		return "", "", err
	}

	return userID.String(), storedPasswordHash, nil
}
