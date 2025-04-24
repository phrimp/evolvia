package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	GrpcPort        string
	JWTSecret       string
	TokenExpiration time.Duration
	ConsulAddress   string
	ServiceName     string
	ServiceID       string
	ServiceAddress  string
}

func init() {
	ServiceConfig = New()
}

var ServiceConfig *Config

func New() *Config {
	tokenExpiry, err := strconv.Atoi(getEnv("TOKEN_EXPIRY_HOURS", "24"))
	if err != nil {
		tokenExpiry = 24
	}

	return &Config{
		Port:            getEnv("PORT", "9200"),
		GrpcPort:        getEnv("GRPC_PORT", "9201"),
		JWTSecret:       getEnv("JWT_SECRET", "your-super-secret-key"),
		TokenExpiration: time.Duration(tokenExpiry) * time.Hour,
		ConsulAddress:   getEnv("CONSUL_ADDRESS", "consul-server:8500"),
		ServiceName:     getEnv("SERVICE_NAME", "middleware"),
		ServiceID:       getEnv("SERVICE_NAME", "middleware") + "-" + getEnv("MIDDLEWARE_HOSTNAME", "1"),
		ServiceAddress:  getEnv("MIDDLEWARE_SERVICE_ADDRESS", "middleware"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
