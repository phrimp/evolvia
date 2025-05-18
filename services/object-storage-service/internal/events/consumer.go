package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"object-storage-service/internal/database/minio"
	"object-storage-service/internal/repository"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Consumer defines the interface for event consumption
type Consumer interface {
	// Start starts the consumer
	Start() error

	// Close closes the consumer
	Close() error
}

// EventConsumer implements the Consumer interface using RabbitMQ
type EventConsumer struct {
	conn             *amqp091.Connection
	channel          *amqp091.Channel
	queueName        string
	exchangeName     string
	fileRepository   *repository.FileRepository
	avatarRepository *repository.AvatarRepository
	shutdown         chan struct{}
	wg               sync.WaitGroup
	enabled          bool
}

// NewEventConsumer creates a new event consumer
func NewEventConsumer(
	rabbitURI string,
	fileRepo *repository.FileRepository,
	avatarRepo *repository.AvatarRepository,
) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consumption is disabled")
		return &EventConsumer{
			fileRepository:   fileRepo,
			avatarRepository: avatarRepo,
			shutdown:         make(chan struct{}),
			enabled:          false,
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

	exchangeName := "storage.events"

	// Declare the exchange
	err = channel.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the queue
	queueName := "storage-service-events"
	_, err = channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &EventConsumer{
		conn:             conn,
		channel:          channel,
		queueName:        queueName,
		exchangeName:     exchangeName,
		fileRepository:   fileRepo,
		avatarRepository: avatarRepo,
		shutdown:         make(chan struct{}),
		enabled:          true,
	}, nil
}

// Start starts consuming events
func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consumption is disabled, not starting consumer")
		return nil
	}

	// Bind the queue to the exchange for all storage events
	for _, routingKey := range []string{
		"file.#",    // All file events
		"avatar.#",  // All avatar events
		"profile.#", // Profile events that might affect avatars
	} {
		err := c.channel.QueueBind(
			c.queueName,    // queue name
			routingKey,     // routing key
			c.exchangeName, // exchange
			false,          // no-wait
			nil,            // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue to exchange: %w", err)
		}
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

	log.Println("Event consumer started")
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
	log.Printf("Processing message with routing key: %s", routingKey)

	switch routingKey {
	case string(EventTypeFileUploaded):
		return c.handleFileUploaded(msg.Body)
	case string(EventTypeFileUpdated):
		return c.handleFileUpdated(msg.Body)
	case string(EventTypeFileDeleted):
		return c.handleFileDeleted(msg.Body)
	case string(EventTypeFileAccessed):
		return c.handleFileAccessed(msg.Body)
	case string(EventTypeAvatarUploaded):
		return c.handleAvatarUploaded(msg.Body)
	case string(EventTypeAvatarUpdated):
		return c.handleAvatarUpdated(msg.Body)
	case string(EventTypeAvatarDeleted):
		return c.handleAvatarDeleted(msg.Body)
	case "profile.deleted":
		return c.handleProfileDeleted(msg.Body)
	default:
		log.Printf("Unknown routing key: %s", routingKey)
		return nil // Acknowledge the message to avoid requeuing
	}
}

// Helper functions to handle different event types

func (c *EventConsumer) handleFileUploaded(body []byte) error {
	var event FileEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal file uploaded event: %w", err)
	}

	log.Printf("File uploaded: ID=%s, Owner=%s, Name=%s", event.FileID, event.OwnerID, event.FileName)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleFileUpdated(body []byte) error {
	var event FileEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal file updated event: %w", err)
	}

	log.Printf("File updated: ID=%s, Owner=%s", event.FileID, event.OwnerID)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleFileDeleted(body []byte) error {
	var event FileEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal file deleted event: %w", err)
	}

	log.Printf("File deleted: ID=%s, Owner=%s", event.FileID, event.OwnerID)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleFileAccessed(body []byte) error {
	var event FileEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal file accessed event: %w", err)
	}

	log.Printf("File accessed: ID=%s, Owner=%s", event.FileID, event.OwnerID)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleAvatarUploaded(body []byte) error {
	var event AvatarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal avatar uploaded event: %w", err)
	}

	log.Printf("Avatar uploaded: ID=%s, User=%s", event.AvatarID, event.UserID)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleAvatarUpdated(body []byte) error {
	var event AvatarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal avatar updated event: %w", err)
	}

	log.Printf("Avatar updated: ID=%s, User=%s", event.AvatarID, event.UserID)
	// No action needed, just logging for now
	return nil
}

func (c *EventConsumer) handleAvatarDeleted(body []byte) error {
	var event AvatarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal avatar deleted event: %w", err)
	}

	log.Printf("Avatar deleted: ID=%s, User=%s", event.AvatarID, event.UserID)
	// No action needed, just logging for now
	return nil
}

// handleProfileDeleted handles profile deletion events to clean up associated avatars
func (c *EventConsumer) handleProfileDeleted(body []byte) error {
	// Parse the profile deleted event
	var event struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal profile deleted event: %w", err)
	}

	if event.UserID == "" {
		return fmt.Errorf("profile deleted event has empty user ID")
	}

	log.Printf("Profile deleted: User=%s, cleaning up avatars", event.UserID)

	// Get all avatars for the user
	ctx := context.Background()
	avatars, err := c.avatarRepository.GetByUserID(ctx, event.UserID)
	if err != nil {
		return fmt.Errorf("failed to get avatars for user: %w", err)
	}

	// Delete each avatar
	for _, avatar := range avatars {
		// Delete avatar file from MinIO
		err := minio.DeleteFile(ctx, avatar.BucketName, avatar.StoragePath)
		if err != nil {
			log.Printf("Error deleting avatar from storage: %v", err)
		}

		// Delete avatar metadata from MongoDB
		err = c.avatarRepository.Delete(ctx, avatar.ID.Hex())
		if err != nil {
			log.Printf("Error deleting avatar metadata: %v", err)
		}
	}

	log.Printf("Deleted %d avatars for user %s", len(avatars), event.UserID)
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
