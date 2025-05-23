package config

import (
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type EmailConfig struct {
	SMTPConfig SMTPConfig
}

func NewEmailConfig() *EmailConfig {
	return &EmailConfig{
		SMTPConfig: loadSMTPConfig(),
	}
}

type Config struct {
	Server           ServerConfig
	GoogleAuth       *GoogleOAuthConfig
	Email            *EmailConfig
	JWT              JWTConfig
	ServiceAccount   *ServiceAccountConfig
	API_KEY          string
	USER_SERVICE_URL string
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type JWTConfig struct {
	Secret        string
	ExpiresIn     time.Duration
	RefreshSecret string
}

type ServiceAccountConfig struct {
	CredentialsJSON []byte
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Endpoint     oauth2.Endpoint
}

func NewGoogleOAuthConfig() *GoogleOAuthConfig {
	return &GoogleOAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
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
