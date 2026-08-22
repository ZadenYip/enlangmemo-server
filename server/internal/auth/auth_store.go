package auth

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/go-sql-driver/mysql"
)

type MySQLUserStore struct {
	db *sql.DB
}

func NewMySQLUserStore(db *sql.DB) *MySQLUserStore {
	return &MySQLUserStore{db: db}
}

func (store *MySQLUserStore) CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (int64, error) {
	const insertUser = `
		INSERT INTO users (login_id, nickname, password_hash) VALUES (?, ?, ?)
	`

	result, err := store.db.ExecContext(ctx, insertUser, loginID, nickname, passwordHash)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		// duplicate entry 1062: https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_dup_entry
		// 利用 unique constraint 防止并发请求导致的重复用户创建。
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return 0, ErrUserAlreadyExists
		}

		return 0, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if userID <= 0 {
		panic("unexpected LastInsertId <= 0, this should never happen")
	}

	return int64(userID), nil
}

func (store *MySQLUserStore) GetPasswordHash(ctx context.Context, loginID string) (int64, string, error) {
	const selectUser = `
		SELECT id, password_hash FROM users WHERE login_id = ?
	`

	var userID int64
	var storedPasswordHash string
	err := store.db.QueryRowContext(ctx, selectUser, loginID).Scan(&userID, &storedPasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrUserNotFound
	}
	if err != nil {
		return 0, "", err
	}
	if userID <= 0 {
		return 0, "", ErrInvalidUserID
	}

	return userID, storedPasswordHash, nil
}

func (store *MySQLUserStore) GetUserProfile(ctx context.Context, userID int64) (UserProfile, error) {
	const selectUser = `
		SELECT login_id, nickname FROM users WHERE id = ?
	`

	profile := UserProfile{UserID: strconv.FormatInt(userID, 10)}
	err := store.db.QueryRowContext(ctx, selectUser, userID).Scan(&profile.LoginID, &profile.Nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return UserProfile{}, ErrUserNotFound
	}
	if err != nil {
		return UserProfile{}, err
	}

	return profile, nil
}
