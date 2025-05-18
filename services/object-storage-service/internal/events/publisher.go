package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// Publisher defines the interface for event publishing
type Publisher interface {
	// File events
	PublishFileUploaded(ctx context.Context, fileID, ownerID, fileName string) error
	PublishFileUpdated(ctx context.Context, fileID, ownerID string) error
	PublishFileDeleted(ctx context.Context, fileID, ownerID string) error
	PublishFileAccessed(ctx context.Context, fileID, ownerID string) error

	// Avatar events
	PublishAvatarUploaded(ctx context.Context, avatarID, userID string) error
	PublishAvatarUpdated(ctx context.Context, avatarID, userID string) error
	PublishAvatarDeleted(ctx context.Context, avatarID, userID string) error

	// Close closes the publisher connection
	Close() error
}

// EventPublisher implements the Publisher interface using RabbitMQ
type EventPublisher struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	exchangeName string
	enabled      bool
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(rabbitURI string) (*EventPublisher, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event publishing is disabled")
		return &EventPublisher{
			enabled: false,
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

	// Declare the exchange
	exchangeName := "storage.events"
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

	return &EventPublisher{
		conn:         conn,
		channel:      channel,
		exchangeName: exchangeName,
		enabled:      true,
	}, nil
}

// publishEvent publishes an event to RabbitMQ
func (p *EventPublisher) publishEvent(ctx context.Context, routingKey string, event interface{}) error {
	if !p.enabled {
		log.Printf("Event publishing is disabled, skipping event: %s", routingKey)
		return nil
	}

	// Convert event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create publishing context with timeout
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Publish the event
	err = p.channel.PublishWithContext(
		pubCtx,
		p.exchangeName, // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("Published event: %s", routingKey)
	return nil
}

// PublishFileUploaded publishes a file uploaded event
func (p *EventPublisher) PublishFileUploaded(ctx context.Context, fileID, ownerID, fileName string) error {
	event := NewFileUploadedEvent(fileID, ownerID, fileName)
	return p.publishEvent(ctx, string(EventTypeFileUploaded), event)
}

// PublishFileUpdated publishes a file updated event
func (p *EventPublisher) PublishFileUpdated(ctx context.Context, fileID, ownerID string) error {
	event := NewFileUpdatedEvent(fileID, ownerID)
	return p.publishEvent(ctx, string(EventTypeFileUpdated), event)
}

// PublishFileDeleted publishes a file deleted event
func (p *EventPublisher) PublishFileDeleted(ctx context.Context, fileID, ownerID string) error {
	event := NewFileDeletedEvent(fileID, ownerID)
	return p.publishEvent(ctx, string(EventTypeFileDeleted), event)
}

// PublishFileAccessed publishes a file accessed event
func (p *EventPublisher) PublishFileAccessed(ctx context.Context, fileID, ownerID string) error {
	event := NewFileAccessedEvent(fileID, ownerID)
	return p.publishEvent(ctx, string(EventTypeFileAccessed), event)
}

// PublishAvatarUploaded publishes an avatar uploaded event
func (p *EventPublisher) PublishAvatarUploaded(ctx context.Context, avatarID, userID string) error {
	event := NewAvatarUploadedEvent(avatarID, userID)
	return p.publishEvent(ctx, string(EventTypeAvatarUploaded), event)
}

// PublishAvatarUpdated publishes an avatar updated event
func (p *EventPublisher) PublishAvatarUpdated(ctx context.Context, avatarID, userID string) error {
	event := NewAvatarUpdatedEvent(avatarID, userID)
	return p.publishEvent(ctx, string(EventTypeAvatarUpdated), event)
}

// PublishAvatarDeleted publishes an avatar deleted event
func (p *EventPublisher) PublishAvatarDeleted(ctx context.Context, avatarID, userID string) error {
	event := NewAvatarDeletedEvent(avatarID, userID)
	return p.publishEvent(ctx, string(EventTypeAvatarDeleted), event)
}

// Close closes the connection to RabbitMQ
func (p *EventPublisher) Close() error {
	if !p.enabled {
		return nil
	}

	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			log.Printf("Error closing RabbitMQ channel: %v", err)
		}
	}

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("error closing RabbitMQ connection: %w", err)
		}
	}

	return nil
}
