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
	Redis    RedisConfig
	RabbitMQ RabbitMQConfig
	Consul   ConsulConfig
	Billing  BillingConfig
}

type ServerConfig struct {
	Port           string
	ServiceName    string
	ServiceAddress string
	ServiceID      string
	GRPCPort       string
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

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type RabbitMQConfig struct {
	URI       string
	QueueName string
	Exchange  string
}

type BillingConfig struct {
	Currency           string
	DefaultGracePeriod time.Duration
	InvoiceRetention   time.Duration
	PaymentRetryLimit  int
	PaymentRetryDelay  time.Duration
	WebhookRetryLimit  int
	WebhookRetryDelay  time.Duration
	TrialPeriodDays    int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           getEnv("PORT", "9240"),
			GRPCPort:       getEnv("GRPC_PORT", "9241"),
			ServiceName:    getEnv("BILLING_SERVICE_NAME", "billing-management-service"),
			ServiceAddress: getEnv("BILLING_SERVICE_ADDRESS", "billing-management-service"),
			ServiceID:      getEnv("BILLING_SERVICE_NAME", "billing-management-service") + "-" + getEnv("HOSTNAME", "billing"),
			ReadTimeout:    getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout:   getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			Host:           getEnv("HOST", "0.0.0.0"),
		},
		Consul: ConsulConfig{
			ConsulAddress: "consul-server:" + getEnv("CONSUL_PORT", "8500"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://root:example@mongodb:27017"),
			Database: getEnv("BILLING_SERVICE_MONGO_DB", "billing_management_service"),
			PoolSize: getEnvAsUint64("MONGODB_POOL_SIZE", 100),
			Timeout:  getEnvAsDuration("MONGODB_TIMEOUT", 10*time.Second),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDR", "redis:6379"),
			Password: getEnv("REDIS_PASSWORD", "example"),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		RabbitMQ: RabbitMQConfig{
			URI:       getEnv("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/"),
			QueueName: getEnv("RABBITMQ_QUEUE", "billing-management-events"),
			Exchange:  getEnv("RABBITMQ_EXCHANGE", "billing.events"),
		},
		Billing: BillingConfig{
			Currency:           getEnv("BILLING_CURRENCY", "USD"),
			DefaultGracePeriod: getEnvAsDuration("BILLING_GRACE_PERIOD", 7*24*time.Hour),
			InvoiceRetention:   getEnvAsDuration("BILLING_INVOICE_RETENTION", 7*365*24*time.Hour), // 7 years
			PaymentRetryLimit:  getEnvAsInt("BILLING_PAYMENT_RETRY_LIMIT", 3),
			PaymentRetryDelay:  getEnvAsDuration("BILLING_PAYMENT_RETRY_DELAY", 24*time.Hour),
			WebhookRetryLimit:  getEnvAsInt("BILLING_WEBHOOK_RETRY_LIMIT", 5),
			WebhookRetryDelay:  getEnvAsDuration("BILLING_WEBHOOK_RETRY_DELAY", 1*time.Hour),
			TrialPeriodDays:    getEnvAsInt("BILLING_TRIAL_PERIOD_DAYS", 14),
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
