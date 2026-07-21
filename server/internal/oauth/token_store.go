package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/server/session"
)

const (
	accessTokenPrefix   = "oauth:access_token:"
	accessTokenTTLHours = 24
)

var ErrAccessTokenNotFound = errors.New("access token not found")

func (s *OAStore) GenAccessToken(ctx context.Context, userID string) (string, error) {
	const maxAttempts = 3
	for range maxAttempts {
		accessToken, err := session.NewID()
		if err != nil {
			return "", err
		}

		ok, err := s.rdb.SetNX(ctx, accessTokenPrefix+accessToken, userID, time.Hour*accessTokenTTLHours).Result()

		if err != nil {
			return "", err
		}

		if ok {
			return accessToken, nil
		}

		s.logger.WarnCtx(ctx, "access token collision, retrying", "accessToken", accessToken)
	}

	return "", errors.New("access token collision")
}

func (s *OAStore) GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error) {
	userID, err := s.rdb.Get(ctx, accessTokenPrefix+accessToken).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrAccessTokenNotFound
		}
		return "", err
	}
	return userID, nil
}
