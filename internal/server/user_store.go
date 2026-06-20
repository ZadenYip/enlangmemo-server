package server

import (
	"context"
	"errors"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore interface {
	CreateUser(ctx context.Context, name string, passwordHash string) (pgtype.UUID, error)
	GetPasswordHash(ctx context.Context, name string) (string, string, error)
}

var errUserAlreadyExists = errors.New("user already exists")
var errUserNotFound = errors.New("user not found")

type pgUserStore struct {
	dbPool *pgxpool.Pool
}

// 参数设置参考
// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#introduction
var argon2Params = argon2id.Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
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

func (store *pgUserStore) GetPasswordHash(ctx context.Context, name string) (string, string, error) {
	const selectUser = `
		SELECT id, password_hash FROM users WHERE name = $1
	`

	var userID pgtype.UUID
	var storedPasswordHash string
	err := store.dbPool.QueryRow(ctx, selectUser, name).Scan(&userID, &storedPasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", errUserNotFound
	}
	if err != nil {
		return "", "", err
	}

	return userID.String(), storedPasswordHash, nil
}
