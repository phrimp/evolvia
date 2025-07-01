package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

func init() {
	ServiceConfig = Load()
}

var ServiceConfig *Config

type Config struct {
	Server   ServerConfig
	MongoDB  MongoDBConfig
	RabbitMQ RabbitMQConfig
	Consul   ConsulConfig
	Coursera CourseraConfig
}

type ServerConfig struct {
	Port           string
	ServiceName    string
	ServiceAddress string
	ServiceID      string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	Host           string
}

type ConsulConfig struct {
	ConsulAddress string
}

type MongoDBConfig struct {
	URI      string
	Database string
	PoolSize uint64
	Timeout  time.Duration
}

type RabbitMQConfig struct {
	URI       string
	QueueName string
	Exchange  string
}

type CourseraConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	BaseURL      string
	AuthURL      string
	TokenURL     string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           getEnv("PORT", "9260"),
			ServiceName:    getEnv("COURSERA_SERVICE_NAME", "coursera-integration-service"),
			ServiceAddress: getEnv("COURSERA_SERVICE_ADDRESS", "coursera-integration-service"),
			ServiceID:      getEnv("COURSERA_SERVICE_NAME", "coursera-integration-service") + "-" + getEnv("HOSTNAME", "coursera"),
			ReadTimeout:    getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout:   getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			Host:           getEnv("HOST", "0.0.0.0"),
		},
		Consul: ConsulConfig{
			ConsulAddress: "consul-server:" + getEnv("CONSUL_PORT", "8500"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://root:example@mongodb:27017"),
			Database: getEnv("COURSERA_SERVICE_MONGO_DB", "coursera_integration"),
			PoolSize: getEnvAsUint64("MONGODB_POOL_SIZE", 100),
			Timeout:  getEnvAsDuration("MONGODB_TIMEOUT", 10*time.Second),
		},
		RabbitMQ: RabbitMQConfig{
			URI:       getEnv("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/"),
			QueueName: getEnv("RABBITMQ_QUEUE", "coursera-integration-events"),
			Exchange:  getEnv("RABBITMQ_EXCHANGE", "coursera.events"),
		},
		Coursera: CourseraConfig{
			ClientID:     getEnv("COURSERA_CLIENT_ID", ""),
			ClientSecret: getEnv("COURSERA_CLIENT_SECRET", ""),
			RedirectURI:  getEnv("COURSERA_REDIRECT_URI", "http://localhost:9260/auth/coursera/callback"),
			BaseURL:      "https://api.coursera.org",
			AuthURL:      "https://accounts.coursera.org/oauth2/v1/auth",
			TokenURL:     "https://accounts.coursera.org/oauth2/v1/token",
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsUint64(key string, defaultValue uint64) uint64 {
	if value, exists := os.LookupEnv(key); exists {
		uint_val, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			log.Printf("error retrieve uint64 env var: %s", err)
			return defaultValue
		}
		return uint_val
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		duration, err := time.ParseDuration(value)
		if err != nil {
			log.Printf("error retrieve duration env var: %s", err)
			return defaultValue
		}
		return duration
	}
	return defaultValue
}
