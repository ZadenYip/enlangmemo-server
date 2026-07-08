package auth

import (
	"context"
	"errors"
)

type UserStore interface {
	CreateUser(ctx context.Context, loginID string, nickname string, passwordHash string) (string, error)
	GetPasswordHash(ctx context.Context, loginID string) (string, string, error)
}

type SessionStore interface {
	Create(ctx context.Context, userID string) (string, error)
	Logout(ctx context.Context, sessionID string) error
}

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")
