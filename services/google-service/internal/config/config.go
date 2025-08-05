package config

import (
	"fmt"
	"log"
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
	Redis      RedisConfig
	RabbitMQ   RabbitMQConfig
	JWT        JWTConfig
	FEADDRESS  string
}

type ServerConfig struct {
	Port         string
	GRPCPort     string // Added gRPC port
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Host         string
}

type ServiceConfig struct {
	Name    string
	Address string
	Port    string
	Version string // Added version info
}

type ConsulConfig struct {
	Address       string
	CheckInterval string // Added health check interval
	CheckTimeout  string // Added health check timeout
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
	Enabled    bool // Added to enable/disable email functionality
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	UseTLS   bool // Added TLS support
}

// Added Redis configuration
type RedisConfig struct {
	Address     string
	Password    string
	DB          int
	Protocol    int
	MaxRetries  int
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// Added RabbitMQ configuration
type RabbitMQConfig struct {
	URI        string
	Host       string
	Port       string
	Username   string
	Password   string
	VHost      string
	Exchange   string
	Queue      string
	RoutingKey string
	Enabled    bool
}

// Added JWT configuration
type JWTConfig struct {
	Secret     string
	ExpiryTime time.Duration
	Issuer     string
}

// Global config instance
var AppConfig *Config

// Initialize loads and returns the configuration
func Initialize() *Config {
	if AppConfig == nil {
		AppConfig = LoadConfig()
	}
	return AppConfig
}

// GetConfig returns the global config instance
func GetConfig() *Config {
	if AppConfig == nil {
		AppConfig = LoadConfig()
	}
	return AppConfig
}

func NewEmailConfig() *EmailConfig {
	enabled, _ := strconv.ParseBool(getEnv("SMTP_ENABLED", "false"))
	fmt.Printf("email: %v", enabled)
	return &EmailConfig{
		SMTPConfig: loadSMTPConfig(),
		Enabled:    enabled,
	}
}

func LoadConfig() *Config {
	log.Println("Loading Google Service configuration...")

	readTimeout, err := strconv.Atoi(getEnv("READ_TIMEOUT", "15"))
	if err != nil {
		log.Printf("Invalid READ_TIMEOUT value, using default: %v", err)
		readTimeout = 15
	}

	writeTimeout, err := strconv.Atoi(getEnv("WRITE_TIMEOUT", "15"))
	if err != nil {
		log.Printf("Invalid WRITE_TIMEOUT value, using default: %v", err)
		writeTimeout = 15
	}

	config := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "9220"),
			GRPCPort:     getEnv("GRPC_PORT", "9221"),
			ReadTimeout:  time.Duration(readTimeout) * time.Second,
			WriteTimeout: time.Duration(writeTimeout) * time.Second,
			Host:         getEnv("HOST", "0.0.0.0"),
		},
		GoogleAuth: loadGoogleOAuthConfig(),
		Service: ServiceConfig{
			Name:    getEnv("SERVICE_NAME", "google-service"),
			Address: getEnv("SERVICE_ADDRESS", "google-service"),
			Port:    getEnv("PORT", "9220"),
			Version: getEnv("SERVICE_VERSION", "1.0.0"),
		},
		Consul: ConsulConfig{
			Address:       getEnv("CONSUL_ADDRESS", "consul-server:8500"),
			CheckInterval: getEnv("CONSUL_CHECK_INTERVAL", "10s"),
			CheckTimeout:  getEnv("CONSUL_CHECK_TIMEOUT", "5s"),
		},
		Email:     NewEmailConfig(),
		Redis:     loadRedisConfig(),
		RabbitMQ:  loadRabbitMQConfig(),
		JWT:       loadJWTConfig(),
		FEADDRESS: getEnv("FE_ADDR", "http://localhost:3000"),
	}

	// Validate required configurations
	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	log.Println("Configuration loaded successfully")
	return config
}

func loadGoogleOAuthConfig() *GoogleOAuthConfig {
	clientID := getEnv("GOOGLE_CLIENT_ID", "")
	if clientID == "" {
		log.Println("Warning: GOOGLE_CLIENT_ID is not set")
	}

	clientSecret := getEnv("GOOGLE_CLIENT_SECRET", "")
	if clientSecret == "" {
		log.Println("Warning: GOOGLE_CLIENT_SECRET is not set")
	}

	return &GoogleOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost/public/google/auth/callback"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/drive.readonly",
			"https://www.googleapis.com/auth/classroom.courses.readonly",
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/gmail.readonly",
		},
		Endpoint: google.Endpoint,
	}
}

func loadRedisConfig() RedisConfig {
	db, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		log.Printf("Invalid REDIS_DB value, using default: %v", err)
		db = 0
	}

	protocol, err := strconv.Atoi(getEnv("REDIS_PROTOCAL", "3"))
	if err != nil {
		log.Printf("Invalid REDIS_PROTOCAL value, using default: %v", err)
		protocol = 3
	}

	maxRetries, err := strconv.Atoi(getEnv("REDIS_MAX_RETRIES", "3"))
	if err != nil {
		log.Printf("Invalid REDIS_MAX_RETRIES value, using default: %v", err)
		maxRetries = 3
	}

	dialTimeout, err := strconv.Atoi(getEnv("REDIS_DIAL_TIMEOUT", "5"))
	if err != nil {
		log.Printf("Invalid REDIS_DIAL_TIMEOUT value, using default: %v", err)
		dialTimeout = 5
	}

	readTimeout, err := strconv.Atoi(getEnv("REDIS_READ_TIMEOUT", "3"))
	if err != nil {
		log.Printf("Invalid REDIS_READ_TIMEOUT value, using default: %v", err)
		readTimeout = 3
	}

	return RedisConfig{
		Address:     getEnv("REDIS_ADDR", "redis:6379"),
		Password:    getEnv("REDIS_PWD", ""),
		DB:          db,
		Protocol:    protocol,
		MaxRetries:  maxRetries,
		DialTimeout: time.Duration(dialTimeout) * time.Second,
		ReadTimeout: time.Duration(readTimeout) * time.Second,
	}
}

func loadRabbitMQConfig() RabbitMQConfig {
	enabled, _ := strconv.ParseBool(getEnv("RABBITMQ_ENABLED", "true"))

	username := getEnv("RABBITMQ_USER", "guest")
	password := getEnv("RABBITMQ_PASSWORD", "guest")
	host := getEnv("RABBITMQ_HOST", "rabbitmq")
	port := getEnv("RABBITMQ_PORT", "5672")
	vhost := getEnv("RABBITMQ_VHOST", "/")

	// Construct URI
	uri := fmt.Sprintf("amqp://%s:%s@%s:%s%s", username, password, host, port, vhost)

	return RabbitMQConfig{
		URI:        getEnv("RABBITMQ_URI", uri),
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		VHost:      vhost,
		Exchange:   getEnv("RABBITMQ_EXCHANGE", "google.events"),
		Queue:      getEnv("RABBITMQ_QUEUE", "google-service-queue"),
		RoutingKey: getEnv("RABBITMQ_ROUTING_KEY", "google.login"),
		Enabled:    enabled,
	}
}

func loadJWTConfig() JWTConfig {
	expiryHours, err := strconv.Atoi(getEnv("TOKEN_EXPIRY_TIME", "24"))
	if err != nil {
		log.Printf("Invalid TOKEN_EXPIRY_TIME value, using default: %v", err)
		expiryHours = 24
	}

	return JWTConfig{
		Secret:     getEnv("JWT_SECRET", ""),
		ExpiryTime: time.Duration(expiryHours) * time.Hour,
		Issuer:     getEnv("JWT_ISSUER", "google-service"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadSMTPConfig() SMTPConfig {
	port := getEnv("SMTP_PORT", "587")
	useTLS, _ := strconv.ParseBool(getEnv("SMTP_USE_TLS", "true"))

	return SMTPConfig{
		Host:     getEnv("SMTP_HOST", ""),
		Port:     port,
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", ""),
		UseTLS:   useTLS,
	}
}

func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if config.GoogleAuth.ClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if config.GoogleAuth.ClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}

	if config.Service.Name == "" {
		return fmt.Errorf("SERVICE_NAME is required")
	}
	if config.Service.Address == "" {
		return fmt.Errorf("SERVICE_ADDRESS is required")
	}

	// Validate timeouts
	if config.Server.ReadTimeout <= 0 {
		return fmt.Errorf("READ timeout must be positive")
	}
	if config.Server.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	// Validate Redis config
	if config.Redis.Address == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}

	// Validate RabbitMQ config if enabled
	if config.RabbitMQ.Enabled && config.RabbitMQ.URI == "" {
		return fmt.Errorf("RABBITMQ_URI is required when RabbitMQ is enabled")
	}

	return nil
}

func (c *Config) IsProduction() bool {
	return getEnv("ENVIRONMENT", "development") == "production"
}

func (c *Config) IsDevelopment() bool {
	return getEnv("ENVIRONMENT", "development") == "development"
}

func (c *Config) GetLogLevel() string {
	return getEnv("LOG_LEVEL", "info")
}

func (c *Config) GetServiceURL() string {
	scheme := "http"
	if c.IsProduction() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%s", scheme, c.Service.Address, c.Service.Port)
}

func (c *Config) GetGRPCServiceURL() string {
	return fmt.Sprintf("%s:%s", c.Service.Address, c.Server.GRPCPort)
}

func (c *Config) GetRedisURL() string {
	if c.Redis.Password != "" {
		return fmt.Sprintf("redis://:%s@%s/%d", c.Redis.Password, c.Redis.Address, c.Redis.DB)
	}
	return fmt.Sprintf("redis://%s/%d", c.Redis.Address, c.Redis.DB)
}

func (c *Config) Print() {
	log.Printf("Configuration:")
	log.Printf("  Service: %s v%s", c.Service.Name, c.Service.Version)
	log.Printf("  Address: %s", c.Service.Address)
	log.Printf("  HTTP Port: %s", c.Server.Port)
	log.Printf("  gRPC Port: %s", c.Server.GRPCPort)
	log.Printf("  Host: %s", c.Server.Host)
	log.Printf("  Read Timeout: %v", c.Server.ReadTimeout)
	log.Printf("  Write Timeout: %v", c.Server.WriteTimeout)
	log.Printf("  Consul: %s", c.Consul.Address)
	log.Printf("  Redis: %s (DB: %d)", c.Redis.Address, c.Redis.DB)
	log.Printf("  RabbitMQ Enabled: %v", c.RabbitMQ.Enabled)
	log.Printf("  Email Enabled: %v", c.Email.Enabled)
	log.Printf("  Frontend Address: %s", c.FEADDRESS)
	log.Printf("  Environment: %s", getEnv("ENVIRONMENT", "development"))
	log.Printf("  Google OAuth Configured: %v", c.GoogleAuth.ClientID != "")
}
