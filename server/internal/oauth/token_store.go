package oauth

import (
	"context"
	_ "embed"
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
var errAccessTokenClientMismatch = errors.New("access token client mismatch")

const (
	revokeAccessTokenNotFound = iota
	revokeAccessTokenRevoked
	revokeAccessTokenClientMismatch
)

//go:embed scripts/revoke_access_token.lua
var revokeAccessTokenLua string

var revokeAccessTokenScript = redis.NewScript(revokeAccessTokenLua)

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

	return parseTokenInfo(tokenInfoJSON)
}

func (s *OAStore) GetUserIDByAccessToken(ctx context.Context, accessToken string) (string, error) {
	tokenInfo, err := s.GetTokenInfoByAccessToken(ctx, accessToken)
	if err != nil {
		return "", err
	}
	return tokenInfo.UserID, nil
}

// RevokeAccessToken 撤销访问令牌
//
// 根据协议要最先验证 clientID 是不是未知客户端，之后才是验证访问令牌是否存在以及 clientID 是否匹配
func (s *OAStore) RevokeAccessToken(ctx context.Context, accessToken, clientID string) error {
	_, err := s.GetClientInfo(ctx, clientID)
	if err != nil {
		// 客户端不存在，这里会返回 errOAClientNotFound
		return err
	}

	result, err := revokeAccessTokenScript.Run(
		ctx,
		s.rdb,
		[]string{accessTokenPrefix + accessToken},
		clientID,
	).Int64()
	if err != nil {
		return err
	}

	switch result {
	case revokeAccessTokenNotFound:
		return ErrAccessTokenNotFound
	case revokeAccessTokenRevoked:
		return nil
	case revokeAccessTokenClientMismatch:
		return errAccessTokenClientMismatch
	default:
		return errors.New("unknown revoke access token script status")
	}
}

func parseTokenInfo(tokenInfoJSON string) (TokenInfo, error) {
	var tokenInfo TokenInfo
	if err := json.Unmarshal([]byte(tokenInfoJSON), &tokenInfo); err != nil {
		return TokenInfo{}, err
	}

	return tokenInfo, nil
}
