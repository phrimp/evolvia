package config

import (
	"os"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	Server     ServerConfig
	GoogleAuth *GoogleOAuthConfig
	Service    ServiceConfig
	Consul     ConsulConfig
	Email      *EmailConfig
	FEADDRESS  string
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Host         string
}

type ServiceConfig struct {
	Name    string
	Address string
	Port    string
}

type ConsulConfig struct {
	Address string
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Endpoint     oauth2.Endpoint
}

type EmailConfig struct {
	SMTPConfig SMTPConfig
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewEmailConfig() *EmailConfig {
	return &EmailConfig{
		SMTPConfig: loadSMTPConfig(),
	}
}

func LoadConfig() *Config {
	readTimeout, _ := strconv.Atoi(getEnv("READ_TIMEOUT", "15"))
	writeTimeout, _ := strconv.Atoi(getEnv("WRITE_TIMEOUT", "15"))

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "9220"),
			ReadTimeout:  time.Duration(readTimeout) * time.Second,
			WriteTimeout: time.Duration(writeTimeout) * time.Second,
		},
		GoogleAuth: &GoogleOAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:9220/public/google/auth/callback"),
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/drive.readonly",
				"https://www.googleapis.com/auth/classroom.courses.readonly",
				"https://www.googleapis.com/auth/calendar.readonly",
				"https://www.googleapis.com/auth/gmail.readonly",
			},
			Endpoint: google.Endpoint,
		},
		Service: ServiceConfig{
			Name:    getEnv("SERVICE_NAME", "google-service"),
			Address: getEnv("SERVICE_ADDRESS", "google-service"),
			Port:    getEnv("PORT", "9220"),
		},
		Consul: ConsulConfig{
			Address: getEnv("CONSUL_ADDRESS", "consul-server:8500"),
		},
		Email:     NewEmailConfig(),
		FEADDRESS: getEnv("FE_ADDR", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadSMTPConfig() SMTPConfig {
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587" // default SMTP port
	}

	return SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     port,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
}
