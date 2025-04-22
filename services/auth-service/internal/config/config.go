package config

import (
	"os"
)

type Config struct {
	Port           string
	ConsulAddress  string
	ServiceName    string
	ServiceID      string
	ServiceAddress string
}

func init() {
	ServiceConfig = New()
}

var ServiceConfig *Config

func New() *Config {
	return &Config{
		Port:           getEnv("PORT", "9000"),
		ConsulAddress:  getEnv("CONSUL_ADDRESS", "consul-server:8500"),
		ServiceName:    getEnv("AUTH_SERVICE_NAME", "auth-service"),
		ServiceID:      getEnv("AUTH_SERVICE_NAME", "auth-service") + "-" + getEnv("AUTH_HOSTNAME", "1"),
		ServiceAddress: getEnv("AUTH_SERVICE_ADDRESS", "AUTH"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
