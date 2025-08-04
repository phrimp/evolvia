package event

import (
	"context"
	"encoding/json"
	"fmt"
	"google-service/internal/repository"
	"google-service/internal/services"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Consumer defines the interface for event consumption
type Consumer interface {
	Start() error
	Close() error
}

// EventConsumer implements the Consumer interface using RabbitMQ
type EventConsumer struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	queueName    string
	emailService *services.EmailService
	otpService   *services.OTPService
	publisher    Publisher
	redisRepo    *repository.RedisRepo
	shutdown     chan struct{}
	wg           sync.WaitGroup
	enabled      bool
}

// Exchange configuration
type ExchangeConfig struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp091.Table
}

// Binding configuration
type BindingConfig struct {
	Exchange   string
	RoutingKey string
}

// UserRegisterEvent represents a user registration event
type UserRegisterEvent struct {
	BaseEvent
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	ProfileData map[string]string `json:"profile_data"`
}

// NewEventConsumer creates a new event consumer
func NewEventConsumer(
	rabbitURI string,
	emailService *services.EmailService,
	otpService *services.OTPService,
	publisher Publisher,
	redisRepo *repository.RedisRepo,
) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consumption is disabled")
		return &EventConsumer{
			emailService: emailService,
			otpService:   otpService,
			publisher:    publisher,
			redisRepo:    redisRepo,
			shutdown:     make(chan struct{}),
			enabled:      false,
		}, nil
	}

	// Connect to RabbitMQ
	conn, err := amqp091.Dial(rabbitURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Set QoS/prefetch
	err = channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return &EventConsumer{
		conn:         conn,
		channel:      channel,
		queueName:    "google-service-events",
		emailService: emailService,
		otpService:   otpService,
		publisher:    publisher,
		redisRepo:    redisRepo,
		shutdown:     make(chan struct{}),
		enabled:      true,
	}, nil
}

// Start starts consuming events
func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consumption is disabled, not starting consumer")
		return nil
	}

	// Define all exchanges this service needs to consume from
	exchanges := []ExchangeConfig{
		{
			Name:       "user-events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
		{
			Name:       "google.events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
		{
			Name:       "auth-events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
	}

	// Declare all exchanges
	for _, exchange := range exchanges {
		err := c.channel.ExchangeDeclare(
			exchange.Name,
			exchange.Type,
			exchange.Durable,
			exchange.AutoDelete,
			exchange.Internal,
			exchange.NoWait,
			exchange.Args,
		)
		if err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}
		log.Printf("Declared exchange: %s", exchange.Name)
	}

	// Declare the queue
	_, err := c.channel.QueueDeclare(
		c.queueName, // name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	log.Printf("Declared queue: %s", c.queueName)

	// Define all bindings
	bindings := []BindingConfig{
		// User events
		{Exchange: "user-events", RoutingKey: "user.registered"},
		{Exchange: "user-events", RoutingKey: "user.#"},

		// Google specific events
		{Exchange: "google.events", RoutingKey: "google.#"},

		// Auth service responses
		{Exchange: "auth-events", RoutingKey: "google.login.response"},
	}

	// Bind the queue to all exchanges with their routing keys
	for _, binding := range bindings {
		err := c.channel.QueueBind(
			c.queueName,        // queue name
			binding.RoutingKey, // routing key
			binding.Exchange,   // exchange
			false,              // no-wait
			nil,                // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue to exchange %s with key %s: %w",
				binding.Exchange, binding.RoutingKey, err)
		}
		log.Printf("Bound queue %s to exchange %s with routing key %s",
			c.queueName, binding.Exchange, binding.RoutingKey)
	}

	// Start consuming messages
	msgs, err := c.channel.Consume(
		c.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.consume(msgs)
	}()

	log.Println("Event consumer started and listening for user registration events")
	return nil
}

// consume handles incoming messages
func (c *EventConsumer) consume(msgs <-chan amqp091.Delivery) {
	for {
		select {
		case <-c.shutdown:
			log.Println("Stopping event consumer")
			return
		case msg, ok := <-msgs:
			if !ok {
				log.Println("Message channel closed, reconnecting...")
				// Attempt to reconnect
				time.Sleep(5 * time.Second)
				return
			}

			// Process the message
			err := c.processMessage(msg)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				// Negative acknowledgement, requeue the message
				if err := msg.Nack(false, true); err != nil {
					log.Printf("Error NACKing message: %v", err)
				}
			} else {
				// Acknowledge the message
				if err := msg.Ack(false); err != nil {
					log.Printf("Error ACKing message: %v", err)
				}
			}
		}
	}
}

// processMessage processes a message based on its routing key
func (c *EventConsumer) processMessage(msg amqp091.Delivery) error {
	routingKey := msg.RoutingKey
	exchange := msg.Exchange

	log.Printf("Processing message from exchange '%s' with routing key: %s", exchange, routingKey)

	switch routingKey {
	case "user.registered":
		return c.handleUserRegistered(msg.Body)
	case "google.login.response":
		return c.handleGoogleLoginResponse(msg.Body)
	default:
		log.Printf("Unknown routing key: %s from exchange: %s", routingKey, exchange)
		return nil // Acknowledge the message to avoid requeuing
	}
}

// handleUserRegistered handles user registration events
func (c *EventConsumer) handleUserRegistered(body []byte) error {
	var event UserRegisterEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal user registered event: %w", err)
	}

	log.Printf("User registered: ID=%s, Username=%s, Email=%s", event.UserID, event.Username, event.Email)

	// Generate OTP for email verification
	otpData, err := c.otpService.GenerateOTP(event.UserID, event.Email)
	if err != nil {
		log.Printf("Failed to generate OTP for user %s: %v", event.UserID, err)
		return fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Prepare email data
	displayName := event.Username
	if name, ok := event.ProfileData["fullname"]; ok && name != "" {
		displayName = name
	}

	emailData := services.EmailData{
		Name:       displayName,
		Email:      event.Email,
		OTPCode:    otpData.Code,
		ExpiryTime: fmt.Sprintf("%d minutes", (otpData.ExpiresAt-otpData.CreatedAt)/60),
		VerifyURL:  fmt.Sprintf("https://your-frontend-url.com/verify-email?user_id=%s&otp=%s", event.UserID, otpData.Code),
	}

	// Send email verification email
	err = c.emailService.SendEmailWithTemplate("email_verification", emailData, []string{event.Email})
	if err != nil {
		log.Printf("Failed to send verification email to %s: %v", event.Email, err)
		// Don't return error here as OTP is already generated
		// User can still verify via API
	} else {
		log.Printf("Verification email sent successfully to %s", event.Email)
	}

	return nil
}

// handleGoogleLoginResponse handles login response events from auth service
func (c *EventConsumer) handleGoogleLoginResponse(body []byte) error {
	var event GoogleLoginResponseEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal google login response event: %w", err)
	}

	log.Printf("Google login response received: RequestID=%s, Success=%t", event.RequestID, event.Success)

	// Store the response in Redis with the request ID as key for pickup by the callback handler
	responseKey := fmt.Sprintf("google-login-response:%s", event.RequestID)

	// Store the response for 5 minutes (enough time for callback to pick it up)
	_, err := c.redisRepo.SaveStructCached(context.Background(), "", responseKey, event, 5)
	if err != nil {
		log.Printf("Failed to store login response in Redis: %v", err)
		return fmt.Errorf("failed to store login response: %w", err)
	}

	log.Printf("Stored login response for request %s in Redis", event.RequestID)
	return nil
}

// Close closes the consumer
func (c *EventConsumer) Close() error {
	if !c.enabled {
		return nil
	}

	// Signal the consumer goroutine to stop
	close(c.shutdown)

	// Wait for the consumer goroutine to finish
	c.wg.Wait()

	// Close the RabbitMQ channel and connection
	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ connection: %w", err)
		}
	}

	return nil
}

