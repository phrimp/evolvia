package configs

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	GinMode              string
	MongoURI             string
	MongoDatabase        string
	MongoProfileDatabase string
	RabbitMQURI          string
	LLMAPIKey            string
	LLMBaseURL           string
	LLMModel             string
	LLMProvider          string
	JWTSecret            string
	ServiceName          string
	ServiceVersion       string
}

var AppConfig *Config

func LoadConfig() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	AppConfig = &Config{
		Port:                 getEnvOrDefault("PORT", "8080"),
		GinMode:              getEnvOrDefault("GIN_MODE", "debug"),
		MongoURI:             getEnvOrDefault("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:        "llm_service",
		MongoProfileDatabase: "profile_service",
		RabbitMQURI:          getEnvOrDefault("RABBITMQ_URI", "amqp://guest:guest@localhost:5672/"),
		LLMAPIKey:            getEnvOrDefault("LLM_API_KEY", ""),
		LLMBaseURL:           getEnvOrDefault("LLM_BASE_URL", "http://localhost:11434/v1"),
		LLMModel:             getEnvOrDefault("LLM_MODEL", "qwen3:1.7b"),
		LLMProvider:          getEnvOrDefault("LLM_PROVIDER", "ollama"),
		JWTSecret:            getEnvOrDefault("JWT_SECRET", "your-jwt-secret-key"),
		ServiceName:          getEnvOrDefault("SERVICE_NAME", "llm-service"),
		ServiceVersion:       getEnvOrDefault("SERVICE_VERSION", "1.0.0"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
