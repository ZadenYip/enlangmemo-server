package sync

import (
	"context"
	"errors"
)

var errInvalidContextUserID = errors.New("invalid userID in context")

func userIDFromCtx(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value("userID").(int64)
	if !ok || userID <= 0 {
		return 0, errInvalidContextUserID
	}
	return userID, nil
}
