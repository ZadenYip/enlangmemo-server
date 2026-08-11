package session

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	id, err := NewID(16)
	require.NoError(t, err)

	const expectedLength = 32
	require.Equal(t, expectedLength, utf8.RuneCountInString(id), "ID length should be %d", expectedLength)
}

func TestNewIDWithBase64RawURL(t *testing.T) {
	id, err := NewIDWithBase64RawURL(DefaultIDLen)
	require.NoError(t, err)

	// base64RawURL 编码后的长度应该是 43 个字符
	const expectedLength = 43
	require.Equal(t, expectedLength, utf8.RuneCountInString(id), "ID length should be %d", expectedLength)
}

func TestNewIDInvalidByteLen(t *testing.T) {
	id, err := NewID(0)

	require.Empty(t, id)
	require.ErrorIs(t, err, ErrInvalidIDByteLen)
}
