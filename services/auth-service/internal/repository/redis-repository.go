package repository

import (
	"auth_service/internal/database/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	redis_v9 "github.com/redis/go-redis/v9"
)

var RedisRepository *RedisRepo

func init() {
	RedisRepository = NewRedisRepo()
}

type RedisRepo struct {
	client *redis_v9.Client
}

func NewRedisRepo() *RedisRepo {
	return &RedisRepo{
		client: redis.Redis_Client,
	}
}

func (us *RedisRepo) SaveStructCached(ctx context.Context, username, key string, model any) (bool, error) {
	val, err := json.Marshal(model)
	if err != nil {
		return false, fmt.Errorf("error saving struct to cache: %s", err)
	}
	err = us.client.Set(ctx, key, val, 24*time.Hour).Err()
	if err != nil {
		return false, fmt.Errorf("error saving struct to cached: %s", err)
	}
	return true, nil
}

func (us *RedisRepo) GetStructCached(ctx context.Context, key, username string, model any) error {
	user_coded, err := us.client.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("error get struct in cache: %s", err)
	}

	return json.Unmarshal(user_coded, model)
}

func (us *RedisRepo) SaveInt(ctx context.Context, username string, value int64, ltime time.Duration, key string) (bool, error) {
	err := us.client.Set(ctx, key, value, ltime*time.Minute).Err()
	if err != nil {
		return false, fmt.Errorf("error saving int64 value to cache: %s", err)
	}
	return true, nil
}

func (us *RedisRepo) GetInt(ctx context.Context, username string, key string) int64 {
	value, err := us.client.Get(ctx, key).Int64()
	if err != nil {
		log.Printf("error get int64 value in cache: %s. Return 0", err)
		return 0
	}
	return value
}
