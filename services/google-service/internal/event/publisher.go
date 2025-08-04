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
	PublishGoogleLogin(ctx context.Context, email, name, avatar, locale string) error
	PublishGoogleLoginRequest(ctx context.Context, email, name, picture, googleID, locale string, profile map[string]string) (*GoogleLoginRequestEvent, error)
	PublishGoogleLoginResponse(ctx context.Context, requestID string, success bool, sessionToken, errorMsg, userID string) error
	PublishEmailVerificationSuccess(ctx context.Context, userID, email string) error
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
	exchangeName := "google.events"
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

func (p *EventPublisher) PublishGoogleLogin(ctx context.Context, email, name, avatar, locale string) error {
	event := NewGoogleLoginEvent(email, name, avatar, locale)
	return p.publishEvent(ctx, string(EventTypeGoogleLogin), event)
}

func (p *EventPublisher) PublishGoogleLoginRequest(ctx context.Context, email, name, picture, googleID, locale string, profile map[string]string) (*GoogleLoginRequestEvent, error) {
	event := NewGoogleLoginRequestEvent(email, name, picture, googleID, locale, profile)
	err := p.publishEvent(ctx, string(EventTypeGoogleLoginRequest), event)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (p *EventPublisher) PublishGoogleLoginResponse(ctx context.Context, requestID string, success bool, sessionToken, errorMsg, userID string) error {
	event := NewGoogleLoginResponseEvent(requestID, success, sessionToken, errorMsg, userID)
	return p.publishEvent(ctx, string(EventTypeGoogleLoginResponse), event)
}

func (p *EventPublisher) PublishEmailVerificationSuccess(ctx context.Context, userID, email string) error {
	event := NewEmailVerificationSuccessEvent(userID, email)
	return p.publishEvent(ctx, string(EventTypeEmailVerificationSuccess), event)
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
