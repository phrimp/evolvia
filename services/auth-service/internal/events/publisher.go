package events

import (
	"context"
	"log"
)

type Publisher interface {
	PublishUserRegister(ctx context.Context, userID, username, email string, profileData map[string]string) error

	// Close closes the publisher and releases resources
	Close() error
}

type EventPublisher struct {
	rabbitMQ *RabbitMQClient
	enabled  bool
}

func NewEventPublisher(rabbitURI string) (*EventPublisher, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event publishing is disabled")
		return &EventPublisher{
			rabbitMQ: nil,
			enabled:  false,
		}, nil
	}

	// Create RabbitMQ client
	client, err := NewRabbitMQClient(rabbitURI)
	if err != nil {
		return nil, err
	}

	// Initialize exchanges and queues
	err = client.setupExchangesAndQueues()
	if err != nil {
		client.Close()
		return nil, err
	}

	return &EventPublisher{
		rabbitMQ: client,
		enabled:  true,
	}, nil
}

func (p *EventPublisher) PublishUserRegister(ctx context.Context, userID, username, email string, profileData map[string]string) error {
	if !p.enabled {
		log.Println("Event publishing is disabled, skipping UserRegisterEvent")
		return nil
	}

	// Create event
	event := NewUserRegisterEvent(userID, username, email, profileData)

	// Serialize to JSON
	eventData, err := event.ToJSON()
	if err != nil {
		return err
	}

	// Publish to RabbitMQ
	err = p.rabbitMQ.PublishEvent("user-events", string(UserRegister), eventData)
	if err != nil {
		return err
	}

	log.Printf("Published UserRegister event for user ID: %s", userID)
	return nil
}

func (p *EventPublisher) PublishUserLogin(ctx context.Context, userID string) error {
	if !p.enabled {
		log.Println("Event publishing is disabled, skipping UserRegisterEvent")
		return nil
	}

	event := NewUserLoginEvent(userID)

	eventData, err := event.ToJSON()
	if err != nil {
		return err
	}

	err = p.rabbitMQ.PublishEvent("user-events", string(UserLogin), eventData)
	if err != nil {
		return err
	}

	log.Printf("Published UserLogin event for user ID: %s", userID)
	return nil
}

// Close releases resources
func (p *EventPublisher) Close() error {
	if !p.enabled || p.rabbitMQ == nil {
		return nil
	}

	return p.rabbitMQ.Close()
}
