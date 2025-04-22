package redis

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Address  string
	Password string
	DB       int
	Protocal int
}

var Redis_Client *redis.Client

func init() {
	config := loadConfig()
	Redis_Client = redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
		Protocol: config.Protocal,
	})
	err := Redis_Client.Conn().Ping(context.Background())
	if err != nil {
		log.Printf("\n Error connect to Redis: %s", err)
	}
}

func loadConfig() *RedisConfig {
	redis_db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	redis_protocal, _ := strconv.Atoi("REDIS_PROTOCAL")
	return &RedisConfig{
		Address:  os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PWD"),
		DB:       redis_db,
		Protocal: redis_protocal,
	}
}
