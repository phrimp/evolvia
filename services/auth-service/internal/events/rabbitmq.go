package events

import (
	"context"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	conn          *amqp.Connection
	channel       *amqp.Channel
	connectionURI string
	isConnected   bool
}

func NewRabbitMQClient(connectionURI string) (*RabbitMQClient, error) {
	client := &RabbitMQClient{
		connectionURI: connectionURI,
		isConnected:   false,
	}

	// Connect to RabbitMQ
	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *RabbitMQClient) connect() error {
	var err error

	// Connect to RabbitMQ server
	c.conn, err = amqp.Dial(c.connectionURI)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Open a channel
	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	// Mark as connected
	c.isConnected = true

	// Set up connection monitoring and auto-reconnect
	go c.monitorConnection()

	return nil
}

func (c *RabbitMQClient) monitorConnection() {
	// Create notification channels
	connCloseChan := make(chan *amqp.Error)
	c.conn.NotifyClose(connCloseChan)

	chanCloseChan := make(chan *amqp.Error)
	c.channel.NotifyClose(chanCloseChan)

	// Monitor for connection/channel closure
	for {
		select {
		case err := <-connCloseChan:
			c.isConnected = false
			log.Printf("RabbitMQ connection closed: %v, attempting to reconnect...", err)
			c.reconnect()
			return // Exit after reconnection is handled
		case err := <-chanCloseChan:
			if c.isConnected {
				log.Printf("RabbitMQ channel closed: %v, reopening...", err)
				c.reopenChannel()
			}
		}
	}
}

func (c *RabbitMQClient) reconnect() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		// Wait before attempting to reconnect
		time.Sleep(backoff)

		// Attempt to reconnect
		err := c.connect()
		if err == nil {
			log.Println("Successfully reconnected to RabbitMQ")

			// Re-initialize exchanges and queues
			if err := c.setupExchangesAndQueues(); err != nil {
				log.Printf("Failed to setup exchanges after reconnection: %v", err)
				continue
			}

			return
		}

		log.Printf("Failed to reconnect to RabbitMQ: %v", err)

		// Increase backoff with a cap
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *RabbitMQClient) reopenChannel() {
	if c.channel != nil {
		c.channel.Close()
	}

	var err error
	c.channel, err = c.conn.Channel()
	if err != nil {
		log.Printf("Failed to reopen channel: %v", err)
		// Connection might be dead, try to reconnect
		c.isConnected = false
		c.reconnect()
		return
	}

	// Reinitialize exchanges and queues
	if err := c.setupExchangesAndQueues(); err != nil {
		log.Printf("Failed to setup exchanges after reopening channel: %v", err)
		c.isConnected = false
		c.reconnect()
		return
	}

	log.Println("Successfully reopened RabbitMQ channel")
}

// setupExchangesAndQueues declares exchanges and queues needed by the client
func (c *RabbitMQClient) setupExchangesAndQueues() error {
	// Declare the user events exchange
	err := c.channel.ExchangeDeclare(
		"user-events", // name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the profile events exchange
	err = c.channel.ExchangeDeclare(
		"profile-events", // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the auth events exchange for responses
	err = c.channel.ExchangeDeclare(
		"auth-events", // name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare auth-events exchange: %w", err)
	}

	// Declare the user.created queue for profile service
	_, err = c.channel.QueueDeclare(
		"user.created.profile", // name
		true,                   // durable
		false,                  // delete when unused
		false,                  // exclusive
		false,                  // no-wait
		nil,                    // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind the queue to the exchange
	err = c.channel.QueueBind(
		"user.created.profile", // queue name
		"user.created",         // routing key
		"user-events",          // exchange
		false,                  // no-wait
		nil,                    // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	return nil
}

func (c *RabbitMQClient) PublishEvent(exchange, routingKey string, body []byte) error {
	if !c.isConnected {
		return fmt.Errorf("cannot publish: not connected to RabbitMQ")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Publish the event
	err := c.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Close closes the connection and channel
func (c *RabbitMQClient) Close() error {
	var err error

	if c.channel != nil {
		err = c.channel.Close()
	}

	if c.conn != nil {
		err = c.conn.Close()
	}

	c.isConnected = false
	return err
}
