package database

import (
	"context"
	"github.com/go-redis/redis/v8"
)

var Redis *redis.Client
var Ctx = context.Background()

func InitRedis(addr, password string) error {
	Redis = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	_, err := Redis.Ping(Ctx).Result()
	return err
}
