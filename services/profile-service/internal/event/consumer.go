package event

import (
	"encoding/json"
	"fmt"
	"log"
	"profile-service/internal/models"
	"profile-service/internal/reporsitory"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
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

	log.Printf("Successfully created default avatar record for user %s", event.UserID)
	return nil
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
