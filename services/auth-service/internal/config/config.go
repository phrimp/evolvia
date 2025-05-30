package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port             string
	GrpcPort         string
	ConsulAddress    string
	ServiceName      string
	ServiceID        string
	ServiceAddress   string
	RabbitMQUSer     string
	RabbitMQPassword string
	RabbitMQPort     string
	JWTSecret        string
	JWTExpired       int64
	FEAddress        string
}

func init() {
	ServiceConfig = New()
}

var ServiceConfig *Config

func New() *Config {
	jwt_expired_str := getEnv("TOKEN_EXPIRY_TIME", "24")
	jwt_expired, _ := strconv.Atoi(jwt_expired_str)

	return &Config{
		Port:             getEnv("PORT", "9100"),
		GrpcPort:         getEnv("GRPC_PORT", "9101"),
		RabbitMQUSer:     getEnv("RABBITMQ_USER", ""),
		RabbitMQPassword: getEnv("RABBITMQ_PASSWORD", ""),
		RabbitMQPort:     getEnv("RABBITMQ_PORT", ""),
		ConsulAddress:    "consul-server:" + getEnv("CONSUL_PORT", "8500"),
		ServiceName:      getEnv("AUTH_SERVICE_NAME", "auth-service"),
		ServiceID:        getEnv("AUTH_SERVICE_NAME", "auth-service") + "-" + getEnv("AUTH_HOSTNAME", "2"),
		ServiceAddress:   getEnv("AUTH_SERVICE_ADDRESS", "auth-service"),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTExpired:       int64(jwt_expired),
		FEAddress:        getEnv("FE_ADDR", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Error Retriving ENV: %s not exist", key)
	return fallback
}
