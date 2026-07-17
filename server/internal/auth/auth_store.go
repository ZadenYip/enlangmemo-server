package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

type MySQLUserStore struct {
	db *sql.DB
}

func NewMySQLUserStore(db *sql.DB) *MySQLUserStore {
	return &MySQLUserStore{db: db}
}

func (store *MySQLUserStore) CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (string, error) {
	const insertUser = `
		INSERT INTO users (id, login_id, nickname, password_hash) VALUES (?, ?, ?, ?)
	`

	userUUID, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	userID := userUUID.String()

	_, err = store.db.ExecContext(ctx, insertUser, userUUID[:], loginID, nickname, passwordHash)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		// duplicate entry 1062: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_dup_entry
		// 利用 unique constraint 防止并发请求导致的重复用户创建。
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return "", ErrUserAlreadyExists
		}

		return "", err
	}

	return userID, nil
}

func (store *MySQLUserStore) GetPasswordHash(ctx context.Context, loginID string) (string, string, error) {
	const selectUser = `
		SELECT id, password_hash FROM users WHERE login_id = ?
	`

	var userUUID uuid.UUID
	var storedPasswordHash string
	err := store.db.QueryRowContext(ctx, selectUser, loginID).Scan(&userUUID, &storedPasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrUserNotFound
	}
	if err != nil {
		return "", "", err
	}

	return userUUID.String(), storedPasswordHash, nil
}
