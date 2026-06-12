package redisclient

import "github.com/redis/go-redis/v9"

func NewClient(redisURL string) *redis.Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(err)
	}
	rdb := redis.NewClient(opt)
	return rdb
}
