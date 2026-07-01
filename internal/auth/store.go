package auth

import (
	"context"
	"errors"
)

type UserStorer interface {
	CreateUser(ctx context.Context, name string, passwordHash string) (string, error)
	GetPasswordHash(ctx context.Context, name string) (string, string, error)
}

type SessionStorer interface {
	Create(ctx context.Context, userID string) (string, error)
}

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")
