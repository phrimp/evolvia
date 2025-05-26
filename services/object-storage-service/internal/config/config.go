package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	MinIO    MinIOConfig
	MongoDB  MongoDBConfig
	RabbitMQ RabbitMQConfig
	Consul   ConsulConfig
}

type ServerConfig struct {
	Port         string
	ServiceName  string
	ServiceID    string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Host         string
}

type MinIOConfig struct {
	Endpoint        string
	PublicEndpoint  string // Add this field for public URL generation
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
	AvatarBucket    string
	FileBucket      string
	DefaultBucket   string
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
	User      string
	Password  string
	Host      string
	Port      string
	VHost     string
}

type ConsulConfig struct {
	Address     string
	ServiceName string
	ServiceID   string
}

// Load loads the configuration from environment variables
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ServiceName:  getEnv("SERVICE_NAME", "object-storage-service"),
			ServiceID:    getEnv("SERVICE_NAME", "object-storage-service") + "-" + getEnv("HOSTNAME", "1"),
			ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			Host:         getEnv("HOST", "0.0.0.0"),
		},
		MinIO: MinIOConfig{
			Endpoint:        getEnv("MINIO_ENDPOINT", "minio:9000"),
			AccessKeyID:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			UseSSL:          getEnvAsBool("MINIO_USE_SSL", false),
			BucketName:      getEnv("MINIO_BUCKET_NAME", "evolvia"),
			Region:          getEnv("MINIO_REGION", "us-east-1"),
			AvatarBucket:    getEnv("MINIO_AVATAR_BUCKET", "avatars"),
			FileBucket:      getEnv("MINIO_FILE_BUCKET", "files"),
			DefaultBucket:   "default",
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://mongodb:27017"),
			Database: getEnv("OBJECT_STORAGE_MONGO_DB", "object_storage"),
			PoolSize: getEnvAsUint64("MONGODB_POOL_SIZE", 100),
			Timeout:  getEnvAsDuration("MONGODB_TIMEOUT", 10*time.Second),
		},
		RabbitMQ: RabbitMQConfig{
			URI:       getEnv("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/"),
			QueueName: getEnv("RABBITMQ_QUEUE", "storage.events"),
			User:      getEnv("RABBITMQ_USER", "guest"),
			Password:  getEnv("RABBITMQ_PASSWORD", "guest"),
			Host:      getEnv("RABBITMQ_HOST", "rabbitmq"),
			Port:      getEnv("RABBITMQ_PORT", "5672"),
			VHost:     getEnv("RABBITMQ_VHOST", "/"),
		},
		Consul: ConsulConfig{
			Address:     getEnv("CONSUL_ADDRESS", "consul-server:8500"),
			ServiceName: getEnv("SERVICE_NAME", "object-storage-service"),
			ServiceID:   getEnv("SERVICE_NAME", "object-storage-service") + "-" + getEnv("HOSTNAME", "1"),
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
		intVal, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("Error converting %s to int: %v", key, err)
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

func getEnvAsUint64(key string, defaultValue uint64) uint64 {
	if value, exists := os.LookupEnv(key); exists {
		intVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			log.Printf("Error converting %s to uint64: %v", key, err)
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("Error converting %s to duration: %v", key, err)
			return defaultValue
		}
		return time.Duration(intVal) * time.Second
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Error converting %s to bool: %v", key, err)
			return defaultValue
		}
		return boolVal
	}
	return defaultValue
}
