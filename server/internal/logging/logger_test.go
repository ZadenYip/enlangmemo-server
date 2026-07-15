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
