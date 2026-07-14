package sso

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/logging"
	"github.com/zadenyip/enlangmemo-server/internal/server/session"
)

type SSOStore interface {
	Create(ctx context.Context, userID string) (string, error)
	GetUserID(ctx context.Context, sessionID string) (string, error)
	Logout(ctx context.Context, sessionID string) (int64, error)
}

var ErrSessionIDCollision = errors.New("session id collision")

const (
	ssoKeyPrefix           = "sso:"
	sessionTimeoutDuration = 8 * time.Hour

	// 8 hours in seconds
	sessionMaxAge = 8 * 3600
)

type RedisSSOStore struct {
	Rdb *redis.Client
	log logging.Logger
}

func (store *RedisSSOStore) Create(ctx context.Context, userID string) (string, error) {
	const createMaxAttempts = 3
	for range createMaxAttempts {
		sessionID, err := session.NewID()
		if err != nil {
			return "", err
		}

		key := ssoKeyPrefix + sessionID
		ok, err := store.Rdb.SetNX(ctx, key, userID, sessionTimeoutDuration).Result()
		if err != nil {
			return "", err
		}
		if ok {
			return sessionID, nil
		} else {
			store.log.WarnCtx(ctx, "session id collision, retrying", "sessionID", sessionID)
		}
	}

	store.log.ErrorCtx(ctx, "failed to create session after max attempts (3)")
	return "", ErrSessionIDCollision
}

func (s *RedisSSOStore) Logout(ctx context.Context, sessionID string) (int64, error) {
	return s.Rdb.Del(ctx, ssoKeyPrefix+sessionID).Result()
}

func (s *RedisSSOStore) GetUserID(ctx context.Context, sessionID string) (string, error) {
	return s.Rdb.Get(ctx, ssoKeyPrefix+sessionID).Result()
}
