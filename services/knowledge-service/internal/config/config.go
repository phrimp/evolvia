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
	Server    ServerConfig
	MongoDB   MongoDBConfig
	Redis     RedisConfig
	RabbitMQ  RabbitMQConfig
	Consul    ConsulConfig
	Knowledge KnowledgeConfig
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

type KnowledgeConfig struct {
	DataDirectory          string
	SkillExtractionEnabled bool
	MinConfidenceScore     float64
	MaxRelatedSkills       int
	SkillCacheExpiry       time.Duration
	AutoReloadData         bool
	ReloadInterval         time.Duration
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           getEnv("PORT", "9340"),
			GRPCPort:       getEnv("GRPC_PORT", "9341"),
			ServiceName:    getEnv("KNOWLEDGE_SERVICE_NAME", "knowledge-service"),
			ServiceAddress: getEnv("KNOWLEDGE_SERVICE_ADDRESS", "knowledge-service"),
			ServiceID:      getEnv("KNOWLEDGE_SERVICE_NAME", "knowledge-service") + "-" + getEnv("HOSTNAME", "knowledge"),
			ReadTimeout:    getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout:   getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			Host:           getEnv("HOST", "0.0.0.0"),
		},
		Consul: ConsulConfig{
			ConsulAddress: "consul-server:" + getEnv("CONSUL_PORT", "8500"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://root:example@mongodb:27017"),
			Database: getEnv("KNOWLEDGE_SERVICE_MONGO_DB", "knowledge_service"),
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
			QueueName: getEnv("RABBITMQ_QUEUE", "knowledge-events"),
			Exchange:  getEnv("RABBITMQ_EXCHANGE", "knowledge.events"),
		},
		Knowledge: KnowledgeConfig{
			DataDirectory:          getEnv("KNOWLEDGE_DATA_DIR", "/data"),
			SkillExtractionEnabled: getEnvAsBool("SKILL_EXTRACTION_ENABLED", true),
			MinConfidenceScore:     getEnvAsFloat("MIN_CONFIDENCE_SCORE", 0.7),
			MaxRelatedSkills:       getEnvAsInt("MAX_RELATED_SKILLS", 10),
			SkillCacheExpiry:       getEnvAsDuration("SKILL_CACHE_EXPIRY", 1*time.Hour),
			AutoReloadData:         getEnvAsBool("AUTO_RELOAD_DATA", false),
			ReloadInterval:         getEnvAsDuration("RELOAD_INTERVAL", 24*time.Hour),
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

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("error retrieve bool env var: %s", err)
			return defaultValue
		}
		return boolVal
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Printf("error retrieve float env var: %s", err)
			return defaultValue
		}
		return floatVal
	}
	return defaultValue
}
