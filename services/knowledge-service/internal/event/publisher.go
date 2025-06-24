package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	PublishSkillEvent(event *SkillEvent) error
	PublishUserSkillEvent(event *UserSkillEvent) error
	PublishSkillExtractionEvent(event *SkillExtractionEvent) error
	PublishSystemEvent(event *SystemEvent) error
	Close() error
}

type EventPublisher struct {
	conn         *amqp091.Connection
	channel      *amqp091.Channel
	exchangeName string
	enabled      bool
}

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
	exchangeName := "knowledge.events"
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

func (p *EventPublisher) publishEvent(ctx context.Context, routingKey string, event any) error {
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

// Skill event publishing
func (p *EventPublisher) PublishSkillEvent(event *SkillEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

// User skill event publishing
func (p *EventPublisher) PublishUserSkillEvent(event *UserSkillEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

// Skill extraction event publishing
func (p *EventPublisher) PublishSkillExtractionEvent(event *SkillExtractionEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

// System event publishing
func (p *EventPublisher) PublishSystemEvent(event *SystemEvent) error {
	ctx := context.Background()
	return p.publishEvent(ctx, event.EventType, event)
}

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
