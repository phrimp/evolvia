package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	MongoDB  MongoDBConfig
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
}

type ServerConfig struct {
	Port         string
	ServiceName  string
	ServiceID    string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Host         string
}

type MongoDBConfig struct {
	URI      string
	Database string
	PoolSize uint64
	Timeout  time.Duration
}

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type RabbitMQConfig struct {
	URI       string
	QueueName string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ServiceName:  getEnv("PROFILE_SERVICE_NAME", "profile-service"),
			ServiceID:    getEnv("PROFILE_SERVICE_NAME", "profile-service") + "-" + getEnv("HOSTNAME", "profile"),
			ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			Host:         getEnv("HOST", "0.0.0.0"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://localhost:27017"),
			Database: getEnv("MONGODB_DATABASE", "profile_service"),
			PoolSize: getEnvAsUint64("MONGODB_POOL_SIZE", 100),
			Timeout:  getEnvAsDuration("MONGODB_TIMEOUT", 10*time.Second),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URI:       getEnv("RABBITMQ_URI", "amqp://guest:guest@localhost:5672/"),
			QueueName: getEnv("RABBITMQ_QUEUE", "profile.events"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		int_val, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("error retrieve int env var: %s", err)
			return defaultValue
		}
		return int_val
	}
	return defaultValue
}

func getEnvAsUint64(key string, defaultValue uint64) uint64 {
	// Implementation here
	return defaultValue // Simplified for this example
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	// Implementation here
	return defaultValue // Simplified for this example
}
