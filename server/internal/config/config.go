package config

import "os"

type Config struct {
	DatabaseURL string
	RedisURL    string
}

func Load() Config {
	return Config{
		// postgres://username:password@localhost:5432/database_name
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:enlangmemo@localhost:5432/enlangmemo"),
		// redis://<user>:<pass>@localhost:6379/<db>
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
