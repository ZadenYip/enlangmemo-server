package interceptor

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/zadenyip/enlangmemo-server/internal/httpauth"
	"github.com/zadenyip/enlangmemo-server/internal/oauth"
)

func NewAuthInterceptor(s oauth.OAStorer) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, r connect.AnyRequest) (connect.AnyResponse, error) {
			token, ok := httpauth.BearerToken(r.Header())
			if !ok {
				return nil, connect.NewError(connect.CodeUnauthenticated,
					errors.New("no token provided"),
				)
			}

			userID, err := s.GetUserIDByAccessToken(ctx, token)
			switch {
			case errors.Is(err, oauth.ErrAccessTokenNotFound):
				return nil, connect.NewError(connect.CodeUnauthenticated,
					errors.New("invalid token"),
				)
			case err != nil:
				return nil, connect.NewError(connect.CodeInternal, nil)
			}

			ctx = context.WithValue(ctx, "userID", userID)
			return next(ctx, r)
		}
	}

	return interceptor
}
