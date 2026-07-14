package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zadenyip/enlangmemo-server/internal/server/session"
)

type OAuthSession struct {
	// Code 作为存进 Redis 的 key 了
	// Code          string `json:"code"`
	RedirectURI   string `json:"redirect_uri"`
	ClientID      string `json:"client_id"`
	CodeChallenge string `json:"code_challenge"`

	UserID string `json:"user_id"`
}

const oaSessionPrefix = "oauth:session:"

var failedToGenerateUniqueAuthCodeErr = errors.New("failed to generate unique auth code after max retries")

// GenCodeStoreSession 会生成一个唯一的授权码，并将其与 OAuthSession 存储在 Redis 中
func (s *OAStore) GenCodeStoreSession(ctx context.Context, authoInfo AuthorizationInfo) (string, error) {
	sessionData := OAuthSession{
		RedirectURI:   authoInfo.redirectURI,
		ClientID:      authoInfo.clientID,
		CodeChallenge: authoInfo.codeChallenge,
		UserID:        authoInfo.userID,
	}

	const maxRetriesConflict = 3
	for range maxRetriesConflict {
		authCode, err := session.NewID()
		if err != nil {
			s.logger.ErrorCtx(ctx, "failed to generate auth code", "err", err)
			return "", err
		}

		dataJSON, err := json.Marshal(sessionData)
		if err != nil {
			s.logger.ErrorCtx(ctx, "failed to marshal oauth session data", "err", err)
			return "", err
		}

		ok, err := s.rdb.SetNX(ctx, oaSessionPrefix+authCode, dataJSON, 10*time.Minute).Result()

		if err != nil {
			s.logger.ErrorCtx(ctx, "failed to store oauth session", "err", err)
			return "", err
		}

		if ok {
			return authCode, nil
		} else {
			s.logger.WarnCtx(ctx, "auth code conflict, trying again")
		}
	}

	s.logger.ErrorCtx(ctx, "failed to generate unique auth code after max retries")
	return "", failedToGenerateUniqueAuthCodeErr
}

var errOASessionNotFound = errors.New("oauth session not found")
var errOASessionExpired = errors.New("oauth session expired")

// ConsumeCodeSession 通过授权码从 Redis 中获取 OAuthSession，并删除 Redis 中的授权码
func (s *OAStore) ConsumeCodeSession(ctx context.Context, authCode string) (OAuthSession, error) {
	result, err := s.rdb.GetDel(ctx, oaSessionPrefix+authCode).Result()
	if errors.Is(err, redis.Nil) {
		return OAuthSession{}, errOASessionNotFound
	}

	if err != nil {
		return OAuthSession{}, err
	}

	var sessionData OAuthSession
	if err := json.Unmarshal([]byte(result), &sessionData); err != nil {
		return OAuthSession{}, err
	}

	return sessionData, nil
}
