package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	config := Load()
	require.NotNil(t, config)

	config.DatabaseURL = "enlangmemo:enlangmemo@tcp(localhost:3306)/enlangmemo?parseTime=true"
	config.RedisURL = "redis://localhost:6379/0"
}
