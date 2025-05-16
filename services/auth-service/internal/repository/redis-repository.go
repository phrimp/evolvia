package repository

import (
	"auth_service/internal/database/redis"
	"auth_service/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func (us *RedisRepo) SaveStructCached(ctx context.Context, username, key string, model any, expired time.Duration) (bool, error) {
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

func (r *RedisRepo) DeleteKey(ctx context.Context, key string) error {
	result := r.client.Del(ctx, key)
	if result.Err() != nil {
		return fmt.Errorf("error deleting key %s: %w", key, result.Err())
	}
	return nil
}

func (r *RedisRepo) SaveProfileCache(ctx context.Context, username string, profile *models.UserWithProfile, ttlMinutes int) error {
	cacheKey := "user-profile:" + username

	// Marshal to JSON
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("error marshaling profile: %w", err)
	}

	// Save to Redis with TTL
	err = r.client.Set(ctx, cacheKey, data, time.Duration(ttlMinutes)*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("error saving profile to cache: %w", err)
	}

	return nil
}

func (r *RedisRepo) GetProfileCache(ctx context.Context, username string) (*models.UserWithProfile, error) {
	cacheKey := "user-profile:" + username

	// Get from Redis
	data, err := r.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("error retrieving profile from cache: %w", err)
	}

	// Unmarshal from JSON
	var profile models.UserWithProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("error unmarshaling profile: %w", err)
	}

	return &profile, nil
}

func (r *RedisRepo) InvalidateProfileCache(ctx context.Context, username string) error {
	cacheKey := "user-profile:" + username

	result := r.client.Del(ctx, cacheKey)
	if result.Err() != nil {
		return fmt.Errorf("error deleting profile cache for user %s: %w", username, result.Err())
	}

	if result.Val() == 0 {
		log.Printf("No cache entry found for user: %s", username)
	}

	return nil
}
