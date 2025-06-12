package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"profile-service/internal/models"
	"profile-service/internal/reporsitory"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Consumer interface {
	Start() error
	Close() error
}

type EventConsumer struct {
	conn              *amqp091.Connection
	channel           *amqp091.Channel
	queueName         string
	profileRepository *reporsitory.ProfileRepository
	shutdown          chan struct{}
	wg                sync.WaitGroup
	enabled           bool
}

type ExchangeConfig struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp091.Table
}

type BindingConfig struct {
	Exchange   string
	RoutingKey string
}

func NewEventConsumer(
	rabbitURI string,
	profileRepo *reporsitory.ProfileRepository,
) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consumption is disabled")
		return &EventConsumer{
			profileRepository: profileRepo,
			shutdown:          make(chan struct{}),
			enabled:           false,
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
		conn:              conn,
		channel:           channel,
		queueName:         "profile-service-events",
		profileRepository: profileRepo,
		shutdown:          make(chan struct{}),
		enabled:           true,
	}, nil
}

func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consumption is disabled, not starting consumer")
		return nil
	}

	// Define all exchanges this service needs to consume from
	exchanges := []ExchangeConfig{
		{
			Name:       "storage.events",
			Type:       "topic",
			Durable:    true,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
		},
		{
			Name:       "user-events",
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
		// Storage events
		{Exchange: "storage.events", RoutingKey: "file.#"},
		{Exchange: "storage.events", RoutingKey: "avatar.#"},

		// User events
		{Exchange: "user-events", RoutingKey: "user.#"},
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

	log.Println("Event consumer started and listening to multiple exchanges")
	return nil
}

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

func (c *EventConsumer) processMessage(msg amqp091.Delivery) error {
	routingKey := msg.RoutingKey
	exchange := msg.Exchange // This tells us which exchange the message came from

	log.Printf("Processing message from exchange '%s' with routing key: %s", exchange, routingKey)

	switch routingKey {
	// Storage events

	// User events
	case "user.registered":
		return c.handleUserRegistered(msg.Body)

	// Profile events
	//	case "profile.created":
	//		return c.handleProfileCreated(msg.Body)
	//	case "profile.deleted":
	//		return c.handleProfileDeleted(msg.Body)
	//	case "profile.updated":
	//		return c.handleProfileUpdated(msg.Body)

	default:
		log.Printf("Unknown routing key: %s from exchange: %s", routingKey, exchange)
		return nil // Acknowledge the message to avoid requeuing
	}
}

func (c *EventConsumer) handleUserRegistered(body []byte) error {
	var event models.UserRegisterEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal user registered event: %w", err)
	}

	log.Printf("User registered: ID=%s, Username=%s, Email=%s", event.UserID, event.Username, event.Email)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	existingProfile, err := c.profileRepository.FindByUserID(ctx, event.UserID)
	if err != nil && err != mongo.ErrNoDocuments {
		log.Printf("Error checking existing profile for user %s: %v", event.UserID, err)
		return fmt.Errorf("failed to check existing profile: %w", err)
	}

	if existingProfile != nil {
		log.Printf("Profile already exists for user %s, skipping creation", event.UserID)
		return nil
	}

	// Extract names from profile data or use defaults
	firstName := "Unknown"
	lastName := "User"

	if event.ProfileData != nil {
		if fname, ok := event.ProfileData["firstName"]; ok && fname != "" {
			firstName = fname
		}
		if lname, ok := event.ProfileData["lastName"]; ok && lname != "" {
			lastName = lname
		}
		// Check for fullname if firstName/lastName not available
		if fullname, ok := event.ProfileData["fullname"]; ok && fullname != "" && firstName == "Unknown" {
			// Split fullname into first and last name
			names := strings.Fields(fullname)
			if len(names) >= 1 {
				firstName = names[0]
			}
			if len(names) >= 2 {
				lastName = strings.Join(names[1:], " ")
			}
		}
	}

	// Create new profile with default values
	profile := &models.Profile{
		UserID: event.UserID,
		PersonalInfo: models.PersonalInfo{
			FirstName:   firstName,
			LastName:    lastName,
			DisplayName: event.Username, // Use username as display name initially
		},
		ContactInfo: models.ContactInfo{
			Email: event.Email,
		},
		PrivacySettings: models.PrivacySettings{
			ProfileVisibility:     models.VisibilityPublic,
			ContactInfoVisibility: models.VisibilityPrivate,
			EducationVisibility:   models.VisibilityPublic,
			ActivityVisibility:    models.VisibilityPrivate,
		},
		EducationalBackground: []models.EducationalBackground{}, // Empty initially
		ProfileCompleteness:   0.0,                              // Will be calculated
		Metadata: models.Metadata{
			CreatedAt: int(time.Now().Unix()),
			UpdatedAt: int(time.Now().Unix()),
		},
	}

	// Calculate initial profile completeness
	profile.ProfileCompleteness = c.calculateCompleteness(profile)

	// Save the profile to database
	createdProfile, err := c.profileRepository.New(ctx, profile)
	if err != nil {
		log.Printf("Failed to create profile for user %s: %v", event.UserID, err)
		return fmt.Errorf("failed to create profile: %w", err)
	}

	log.Printf("Successfully created profile for user %s with ID %s (%.1f%% complete)",
		event.UserID, createdProfile.ID.Hex(), createdProfile.ProfileCompleteness)

	// if c.eventPublisher != nil {
	//	profileEvent := &models.ProfileEvent{
	//		EventType: models.EventTypeProfileCreated,
	//		ProfileID: createdProfile.ID.Hex(),
	//		UserID:    createdProfile.UserID,
	//		Timestamp: int(time.Now().Unix()),
	//		NewValues: map[string]any{
	//			"firstName":           createdProfile.PersonalInfo.FirstName,
	//			"lastName":            createdProfile.PersonalInfo.LastName,
	//			"email":               createdProfile.ContactInfo.Email,
	//			"profileCompleteness": createdProfile.ProfileCompleteness,
	//		},
	//	}

	//	if err := c.eventPublisher.PublishProfileEvent(profileEvent); err != nil {
	//		log.Printf("Warning: Failed to publish profile created event for user %s: %v", event.UserID, err)
	//		// Don't return error here as profile creation was successful
	//	}
	//}

	return nil
}

// Helper method to calculate profile completeness
func (c *EventConsumer) calculateCompleteness(profile *models.Profile) float64 {
	totalFields := 0
	completedFields := 0

	// Personal Info fields (6 total)
	totalFields += 6
	if profile.PersonalInfo.FirstName != "" && profile.PersonalInfo.FirstName != "Unknown" {
		completedFields++
	}
	if profile.PersonalInfo.LastName != "" && profile.PersonalInfo.LastName != "User" {
		completedFields++
	}
	if profile.PersonalInfo.DisplayName != "" {
		completedFields++
	}
	if profile.PersonalInfo.DateOfBirth != 0 {
		completedFields++
	}
	if profile.PersonalInfo.Gender != "" {
		completedFields++
	}
	if profile.PersonalInfo.Biography != "" {
		completedFields++
	}

	// Contact Info fields (4 total)
	totalFields += 4
	if profile.ContactInfo.Email != "" {
		completedFields++
	}
	if profile.ContactInfo.Phone != "" {
		completedFields++
	}
	if profile.ContactInfo.AlternativeEmail != "" {
		completedFields++
	}
	if profile.ContactInfo.Address != nil && profile.ContactInfo.Address.Country != "" {
		completedFields++
	}

	// Educational Background (1 field)
	totalFields += 1
	if len(profile.EducationalBackground) > 0 {
		completedFields++
	}

	if totalFields == 0 {
		return 0.0
	}

	return float64(completedFields) / float64(totalFields) * 100.0
}

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
