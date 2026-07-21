package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/zadenyip/enlangmemo-server/internal/server/session"
)

const (
	accessTokenPrefix   = "oauth:access_token:"
	accessTokenTTLHours = 24
)

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
