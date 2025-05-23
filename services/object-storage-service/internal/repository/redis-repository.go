package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"object-storage-service/internal/database/redis"
	"time"

	redis_v9 "github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	client *redis_v9.Client
}

func NewRedisRepo() *RedisRepo {
	return &RedisRepo{
		client: redis.Redis_Client,
	}
}

func (us *RedisRepo) SaveStructCached(ctx context.Context, signature, key string, model any, expired time.Duration) (bool, error) {
	val, err := json.Marshal(model)
	key = key + signature
	if err != nil {
		return false, fmt.Errorf("error saving struct to cache: %s", err)
	}
	err = us.client.Set(ctx, key, val, expired*time.Hour).Err()
	if err != nil {
		return false, fmt.Errorf("error saving struct to cached: %s", err)
	}
	return true, nil
}

func (us *RedisRepo) GetStructCached(ctx context.Context, key, signature string, model any) error {
	key = key + signature
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

func (r *RedisRepo) DeleteKey(ctx context.Context, key string) error {
	result := r.client.Del(ctx, key)
	if result.Err() != nil {
		return fmt.Errorf("error deleting key %s: %w", key, result.Err())
	}
	return nil
}
