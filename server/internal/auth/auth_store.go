package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-sql-driver/mysql"
)

type MySQLUserStore struct {
	db *sql.DB
}

func NewMySQLUserStore(db *sql.DB) *MySQLUserStore {
	return &MySQLUserStore{db: db}
}

func (store *MySQLUserStore) CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (string, error) {
	const insertUser = `
		INSERT INTO users (login_id, nickname, password_hash) VALUES (?, ?, ?)
	`

	result, err := store.db.ExecContext(ctx, insertUser, loginID, nickname, passwordHash)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		// duplicate entry 1062: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_dup_entry
		// 利用 unique constraint 防止并发请求导致的重复用户创建。
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return "", ErrUserAlreadyExists
		}

		return "", err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(userID, 10), nil
}

func (store *MySQLUserStore) GetPasswordHash(ctx context.Context, loginID string) (string, string, error) {
	const selectUser = `
		SELECT id, password_hash FROM users WHERE login_id = ?
	`

	var userID uint64
	var storedPasswordHash string
	err := store.db.QueryRowContext(ctx, selectUser, loginID).Scan(&userID, &storedPasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrUserNotFound
	}
	if err != nil {
		return "", "", err
	}

	return strconv.FormatUint(userID, 10), storedPasswordHash, nil
}

func (store *MySQLUserStore) GetUserProfile(ctx context.Context, userID string) (UserProfile, error) {
	parsedUserID, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return UserProfile{}, fmt.Errorf("%w: %v", ErrInvalidUserID, err)
	}

	const selectUser = `
		SELECT login_id, nickname FROM users WHERE id = ?
	`

	profile := UserProfile{UserID: strconv.FormatUint(parsedUserID, 10)}
	err = store.db.QueryRowContext(ctx, selectUser, parsedUserID).Scan(&profile.LoginID, &profile.Nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return UserProfile{}, ErrUserNotFound
	}
	if err != nil {
		return UserProfile{}, err
	}

	return profile, nil
}
