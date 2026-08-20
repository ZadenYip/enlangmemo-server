package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCtxWithoutUserID(t *testing.T) {
	ctx := t.Context()
	_, err := userIDFromCtx(ctx)
	require.NotNil(t, err)
}

func TestCtxWithNegativeUserID(t *testing.T) {
	ctx := t.Context()
	ctx = context.WithValue(ctx, "userID", int64(-1))
	_, err := userIDFromCtx(ctx)
	require.NotNil(t, err)
}

func TestCtxWithWrongTypeUserID(t *testing.T) {
	ctx := t.Context()
	ctx = context.WithValue(ctx, "userID", "not an int64")
	_, err := userIDFromCtx(ctx)
	require.NotNil(t, err)
}

func TestCtxWithValidUserID(t *testing.T) {
	ctx := t.Context()
	ctx = context.WithValue(ctx, "userID", int64(123))
	userID, err := userIDFromCtx(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(123), userID)
}
