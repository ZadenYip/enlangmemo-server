package auth

import (
	"context"
	"errors"
)

type UserStore interface {
	CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (int64, error)
	GetPasswordHash(ctx context.Context, loginID string) (int64, string, error)
}

type UserProfile struct {
	UserID   string
	LoginID  string
	Nickname string
}

type SSOStore interface {
	Create(ctx context.Context, userID int64) (string, error)
	Logout(ctx context.Context, sessionID string) (int64, error)
}

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")
var ErrInvalidUserID = errors.New("invalid user ID")
