package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"middleware/internal/database/redis"
	"time"

	redis_v9 "github.com/redis/go-redis/v9"
)

type RedisService struct {
	client *redis_v9.Client
}

var Redis_service *RedisService

func init() {
	Redis_service = NewRedisService()
}

func NewRedisService() *RedisService {
	return &RedisService{
		client: redis.Redis_Client,
	}
}

func (us *RedisService) SaveStructCached(ctx context.Context, key string, model any, expired time.Duration) (bool, error) {
	val, err := json.Marshal(model)
	if err != nil {
		return false, fmt.Errorf("error saving struct to cache: %s", err)
	}
	err = us.client.Set(ctx, key, val, expired*time.Hour).Err()
	if err != nil {
		return false, fmt.Errorf("error saving struct to cached: %s", err)
	}
	return true, nil
}

func (us *RedisService) GetStructCached(ctx context.Context, key string, model any) error {
	user_coded, err := us.client.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("error get struct in cache: %s", err)
	}

	return json.Unmarshal(user_coded, model)
}

func (us *RedisService) SaveInt(ctx context.Context, value int64, ltime time.Duration, key string) (bool, error) {
	err := us.client.Set(ctx, key, value, ltime*time.Minute).Err()
	if err != nil {
		return false, fmt.Errorf("error saving int64 value to cache: %s", err)
	}
	return true, nil
}

func (us *RedisService) GetInt(ctx context.Context, key string) int64 {
	value, err := us.client.Get(ctx, key).Int64()
	if err != nil {
		log.Printf("error get int64 value in cache: %s. Return 0", err)
		return 0
	}
	return value
}
