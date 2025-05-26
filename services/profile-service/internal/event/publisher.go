package event

import (
	"encoding/json"
	"fmt"
	"log"
	"profile-service/internal/models"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	PublishProfileEvent(event *models.ProfileEvent) error
	Close() error
}

type EventPublisher struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	exchange string
	enabled  bool
}

func NewEventPublisher(rabbitURI string) (*EventPublisher, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event publishing is disabled")
		return &EventPublisher{
			exchange: "profile.events",
			enabled:  false,
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
	exchange := "profile.events"
	err = channel.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	log.Printf("Event publisher initialized with exchange: %s", exchange)

	return &EventPublisher{
		conn:     conn,
		channel:  channel,
		exchange: exchange,
		enabled:  true,
	}, nil
}

func (p *EventPublisher) PublishProfileEvent(event *models.ProfileEvent) error {
	if !p.enabled {
		log.Printf("Event publishing disabled, skipping event: %s", event.EventType)
		return nil
	}

	// Convert event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Determine routing key based on event type
	routingKey := string(event.EventType)

	// Publish the event
	err = p.channel.Publish(
		p.exchange, // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // Make message persistent
			Timestamp:    time.Now(),
			Body:         eventData,
			Headers: amqp091.Table{
				"event_type": string(event.EventType),
				"profile_id": event.ProfileID,
				"user_id":    event.UserID,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("Published event: %s for profile: %s", event.EventType, event.ProfileID)
	return nil
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

type MockPublisher struct {
	Events []models.ProfileEvent
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		Events: make([]models.ProfileEvent, 0),
	}
}

func (m *MockPublisher) PublishProfileEvent(event *models.ProfileEvent) error {
	m.Events = append(m.Events, *event)
	return nil
}

func (m *MockPublisher) Close() error {
	return nil
}

func (m *MockPublisher) GetEvents() []models.ProfileEvent {
	return m.Events
}

func (m *MockPublisher) ClearEvents() {
	m.Events = make([]models.ProfileEvent, 0)
}
