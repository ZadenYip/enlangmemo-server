package oauth

import (
	"context"
	"encoding/json"
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

func (s *OAStore) GenAccessToken(ctx context.Context, clientID, userID string) (string, error) {
	const maxAttempts = 3
	for range maxAttempts {
		accessToken, err := session.NewID()
		if err != nil {
			return "", err
		}

		tokenInfo, err := json.Marshal(TokenInfo{
			UserID:   userID,
			ClientID: clientID,
		})

		if err != nil {
			return "", err
		}

		ok, err := s.rdb.SetNX(ctx, accessTokenPrefix+accessToken, tokenInfo, time.Hour*accessTokenTTLHours).Result()

		if err != nil {
			return "", err
		}

		if ok {
			return accessToken, nil
		}

		s.logger.WarnCtx(ctx, "access token collision, retrying")
	}

	return "", errors.New("access token collision")
}

type TokenInfo struct {
	UserID   string `json:"user_id"`
	ClientID string `json:"client_id"`
}

func (s *OAStore) GetTokenInfoByAccessToken(ctx context.Context, accessToken string) (TokenInfo, error) {
	tokenInfoJSON, err := s.rdb.Get(ctx, accessTokenPrefix+accessToken).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return TokenInfo{}, ErrAccessTokenNotFound
		}
		return TokenInfo{}, err
	}

	var tokenInfo TokenInfo
	if err := json.Unmarshal([]byte(tokenInfoJSON), &tokenInfo); err != nil {
		return TokenInfo{}, err
	}

	return tokenInfo, nil
}

func (s *OAStore) GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error) {
	tokenInfo, err := s.GetTokenInfoByAccessToken(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return tokenInfo.UserID, nil
}
