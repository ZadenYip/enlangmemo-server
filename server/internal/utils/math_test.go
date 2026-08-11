package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAbs(t *testing.T) {
	const positive int64 = 42
	if got := Abs(positive); got != positive {
		require.Equal(t, positive, got, "Abs(%d) = %d; want %d", positive, got, positive)
	}

	const negative int64 = -42
	if got := Abs(negative); got != -negative {
		require.Equal(t, -negative, got, "Abs(%d) = %d; want %d", negative, got, -negative)
	}

	const zero int64 = 0
	if got := Abs(zero); got != zero {
		require.Equal(t, zero, got, "Abs(%d) = %d; want %d", zero, got, zero)
	}
}
