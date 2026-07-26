package logging

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerLogInfo(t *testing.T) {
	logger := NewServerLog()

	infoLogger := logger.Info()

	require.NotNil(t, infoLogger)
	require.IsType(t, &slog.Logger{}, infoLogger)
	require.Same(t, logger.infoLog, infoLogger)
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "empty value",
			value: "",
			want:  "***",
		},
		{
			name:  "short value",
			value: "short",
			want:  "***",
		},
		{
			name:  "boundary value",
			value: "12345678",
			want:  "***",
		},
		{
			name:  "long value",
			value: "1234567890abcdef",
			want:  "1234***cdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MaskSecret(tt.value))
		})
	}
}

func TestMaskSecretWith(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		prefix int
		suffix int
		want   string
	}{
		{
			name:   "empty value",
			value:  "",
			prefix: 2,
			suffix: 2,
			want:   "***",
		},
		{
			name:   "shorter than visible length",
			value:  "abc",
			prefix: 2,
			suffix: 2,
			want:   "***",
		},
		{
			name:   "equal to visible length",
			value:  "abcd",
			prefix: 2,
			suffix: 2,
			want:   "***",
		},
		{
			name:   "longer than visible length",
			value:  "abcdefghij",
			prefix: 2,
			suffix: 3,
			want:   "ab***hij",
		},
		{
			name:   "prefix only",
			value:  "abcdefghij",
			prefix: 3,
			suffix: 0,
			want:   "abc***",
		},
		{
			name:   "suffix only",
			value:  "abcdefghij",
			prefix: 0,
			suffix: 3,
			want:   "***hij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MaskSecretWith(tt.value, tt.prefix, tt.suffix))
		})
	}
}
