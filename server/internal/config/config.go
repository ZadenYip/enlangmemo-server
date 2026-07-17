package config

import "os"

type Config struct {
	DatabaseURL string
	RedisURL    string
}

func Load() Config {
	return Config{
		// username:password@tcp(localhost:3306)/database_name?parseTime=true
		DatabaseURL: getEnv("DATABASE_URL", "enlangmemo:enlangmemo@tcp(localhost:3306)/enlangmemo?parseTime=true"),
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
