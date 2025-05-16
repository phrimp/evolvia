package events

import (
	"auth_service/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer interface {
	Start() error

	Close() error
}

type EventConsumer struct {
	rabbitMQ  *RabbitMQClient
	redisRepo *repository.RedisRepo
	enabled   bool
	closeCh   chan struct{}
}

func NewEventConsumer(rabbitURI string, redisRepo *repository.RedisRepo) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consuming is disabled")
		return &EventConsumer{
			rabbitMQ:  nil,
			redisRepo: redisRepo,
			enabled:   false,
			closeCh:   make(chan struct{}),
		}, nil
	}

	client, err := NewRabbitMQClient(rabbitURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	return &EventConsumer{
		rabbitMQ:  client,
		redisRepo: redisRepo,
		enabled:   true,
		closeCh:   make(chan struct{}),
	}, nil
}

func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consuming is disabled, not starting consumer")
		return nil
	}

	// Set up the exchanges and queues
	if err := c.setupProfileEventConsumer(); err != nil {
		return fmt.Errorf("failed to set up profile event consumer: %w", err)
	}

	log.Println("Event consumer started successfully")
	return nil
}

func (c *EventConsumer) setupProfileEventConsumer() error {
	// Declare the exchange
	err := c.rabbitMQ.channel.ExchangeDeclare(
		"profile-events", // name
		"topic",          // type
		true,             // durable
		false,            // auto-delete
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the queue
	q, err := c.rabbitMQ.channel.QueueDeclare(
		"auth.cache.invalidation", // name
		true,                      // durable
		false,                     // delete when unused
		false,                     // exclusive
		false,                     // no-wait
		nil,                       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind the queue to the exchange
	err = c.rabbitMQ.channel.QueueBind(
		q.Name,            // queue name
		"profile.updated", // routing key
		"profile-events",  // exchange
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Start consuming messages
	msgs, err := c.rabbitMQ.channel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Start a goroutine to handle messages
	go c.handleProfileEvents(msgs)

	return nil
}

func (c *EventConsumer) handleProfileEvents(msgs <-chan amqp.Delivery) {
	for {
		select {
		case <-c.closeCh:
			// Consumer is being closed
			return
		case msg, ok := <-msgs:
			if !ok {
				// Channel closed, try to reconnect
				log.Println("RabbitMQ channel closed, attempting to reconnect...")
				for {
					// Try to set up the consumer again
					err := c.setupProfileEventConsumer()
					if err == nil {
						break
					}
					log.Printf("Failed to reconnect: %v, retrying in 5 seconds...", err)
					time.Sleep(5 * time.Second)
				}
				return
			}

			// Process the message
			c.processProfileEvent(msg)
		}
	}
}

func (c *EventConsumer) processProfileEvent(msg amqp.Delivery) {
	defer func() {
		// Ensure the message is acknowledged even if processing fails
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	// Check event type from routing key
	if msg.RoutingKey != "profile.updated" {
		log.Printf("Received unexpected routing key: %s", msg.RoutingKey)
		return
	}

	// Parse the event
	var event ProfileUpdatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("Error parsing profile updated event: %v", err)
		return
	}

	// Invalidate the cache
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.redisRepo.InvalidateProfileCache(ctx, event.Username)
	if err != nil {
		log.Printf("Error invalidating profile cache for user %s: %v", event.Username, err)
		return
	}

	log.Printf("Successfully invalidated profile cache for user: %s", event.Username)
}

func (c *EventConsumer) Close() error {
	if !c.enabled {
		return nil
	}

	// Signal the consumer goroutine to close
	close(c.closeCh)

	// Close the RabbitMQ client
	if c.rabbitMQ != nil {
		return c.rabbitMQ.Close()
	}

	return nil
}
