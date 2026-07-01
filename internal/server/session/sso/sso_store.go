package sso

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/server/session"
)

type SSOStore interface {
	Create(ctx context.Context, userID string) (string, error)
	GetUserID(ctx context.Context, sessionID string) (string, error)
	Delete(ctx context.Context, sessionID string) error
}

var ErrSessionIDCollision = errors.New("session id collision")

const (
	ssoKeyPrefix           = "sso:"
	sessionTimeoutDuration = 8 * time.Hour

	// 8 hours in seconds
	sessionMaxAge = 8 * 3600

	createMaxAttempts = 3
)

type RedisSSOStore struct {
	Rds *redis.Client
}

func (store *RedisSSOStore) Create(ctx context.Context, userID string) (string, error) {
	for range createMaxAttempts {
		sessionID, err := session.NewToken()
		if err != nil {
			return "", err
		}

		key := ssoKeyPrefix + sessionID
		ok, err := store.Rds.SetNX(ctx, key, userID, sessionTimeoutDuration).Result()
		if err != nil {
			return "", err
		}
		if ok {
			return sessionID, nil
		}
	}

	return "", ErrSessionIDCollision
}

func (store *RedisSSOStore) Logout(ctx context.Context, sessionID string) error {
	return store.Rds.Del(ctx, ssoKeyPrefix+sessionID).Err()
}

func (store *RedisSSOStore) GetUserID(ctx context.Context, sessionID string) (string, error) {
	return store.Rds.Get(ctx, ssoKeyPrefix+sessionID).Result()
}

func (store *RedisSSOStore) Delete(ctx context.Context, sessionID string) error {
	return store.Rds.Del(ctx, ssoKeyPrefix+sessionID).Err()
}
