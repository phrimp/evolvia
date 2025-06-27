package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Consumer interface {
	Start() error
	Close() error
}

type PaymentHandler interface {
	HandlePaymentSuccess(ctx context.Context, subscriptionID bson.ObjectID, orderCode string) error
	HandlePaymentFailed(ctx context.Context, subscriptionID bson.ObjectID, orderCode string) error
	HandlePaymentCancelled(ctx context.Context, subscriptionID bson.ObjectID, orderCode string) error
	HandlePaymentTimeout(ctx context.Context, subscriptionID bson.ObjectID, orderCode string) error
}

type EventConsumer struct {
	conn           *amqp091.Connection
	channel        *amqp091.Channel
	queueName      string
	paymentHandler PaymentHandler
	enabled        bool
}

type PaymentEventData struct {
	Type           string  `json:"type"`
	OrderCode      string  `json:"orderCode"`
	SubscriptionID string  `json:"subscription_id"`
	Amount         float64 `json:"amount"`
	Description    string  `json:"description"`
	Timestamp      string  `json:"timestamp"`
	Data           any     `json:"data,omitempty"`
}

func NewEventConsumer(rabbitURI string, paymentHandler PaymentHandler) (*EventConsumer, error) {
	if rabbitURI == "" {
		log.Println("Warning: RabbitMQ URI is empty, event consumption is disabled")
		return &EventConsumer{
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
	exchangeName := "billing.events"
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
	queueName := "billing-service-payment-events"
	queue, err := channel.QueueDeclare(
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

	// Bind the queue to handle payment processing events
	err = channel.QueueBind(
		queue.Name,           // queue name
		"payment.processing", // routing key
		exchangeName,         // exchange
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	return &EventConsumer{
		conn:           conn,
		channel:        channel,
		queueName:      queue.Name,
		paymentHandler: paymentHandler,
		enabled:        true,
	}, nil
}

func (c *EventConsumer) Start() error {
	if !c.enabled {
		log.Println("Event consumption is disabled")
		return nil
	}

	// Set QoS
	err := c.channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
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
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Process messages in a goroutine
	go func() {
		for msg := range msgs {
			if err := c.processMessage(msg); err != nil {
				log.Printf("Failed to process message: %v", err)
				msg.Nack(false, true) // Nack and requeue
			} else {
				msg.Ack(false) // Acknowledge message
			}
		}
	}()

	log.Println("Payment event consumer started, waiting for messages...")
	return nil
}

func (c *EventConsumer) processMessage(msg amqp091.Delivery) error {
	log.Printf("Received message with routing key: %s", msg.RoutingKey)

	switch msg.RoutingKey {
	case "payment.processing":
		return c.handlePaymentEvent(msg.Body)
	default:
		log.Printf("Unknown routing key: %s", msg.RoutingKey)
		return nil // Don't requeue unknown message types
	}
}

func (c *EventConsumer) handlePaymentEvent(body []byte) error {
	var paymentEvent PaymentEventData
	if err := json.Unmarshal(body, &paymentEvent); err != nil {
		return fmt.Errorf("failed to unmarshal payment event: %w", err)
	}

	log.Printf("Processing payment event: %s for subscription %s (order: %s)",
		paymentEvent.Type, paymentEvent.SubscriptionID, paymentEvent.OrderCode)

	if paymentEvent.SubscriptionID == "" {
		log.Printf("No subscription ID in payment event, skipping")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	subscriptionObjectID, err := bson.ObjectIDFromHex(paymentEvent.SubscriptionID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID format: %w", err)
	}

	switch paymentEvent.Type {
	case "PAYMENT_SUCCESS":
		return c.handlePaymentSuccess(ctx, subscriptionObjectID, paymentEvent)
	case "PAYMENT_FAILED":
		return c.handlePaymentFailed(ctx, subscriptionObjectID, paymentEvent)
	case "PAYMENT_CANCELLED":
		return c.handlePaymentCancelled(ctx, subscriptionObjectID, paymentEvent)
	case "PAYMENT_TIMEOUT":
		return c.handlePaymentTimeout(ctx, subscriptionObjectID, paymentEvent)
	default:
		log.Printf("Unknown payment event type: %s", paymentEvent.Type)
		return nil
	}
}

func (c *EventConsumer) handlePaymentSuccess(ctx context.Context, subscriptionID bson.ObjectID, event PaymentEventData) error {
	log.Printf("Handling payment success for subscription: %s", subscriptionID.Hex())

	err := c.paymentHandler.HandlePaymentSuccess(ctx, subscriptionID, event.OrderCode)
	if err != nil {
		return fmt.Errorf("failed to handle payment success: %w", err)
	}

	log.Printf("Successfully processed payment success for subscription %s", subscriptionID.Hex())
	return nil
}

func (c *EventConsumer) handlePaymentFailed(ctx context.Context, subscriptionID bson.ObjectID, event PaymentEventData) error {
	log.Printf("Handling payment failed for subscription: %s", subscriptionID.Hex())

	err := c.paymentHandler.HandlePaymentFailed(ctx, subscriptionID, event.OrderCode)
	if err != nil {
		return fmt.Errorf("failed to handle payment failed: %w", err)
	}

	log.Printf("Successfully processed payment failed for subscription %s", subscriptionID.Hex())
	return nil
}

func (c *EventConsumer) handlePaymentCancelled(ctx context.Context, subscriptionID bson.ObjectID, event PaymentEventData) error {
	log.Printf("Handling payment cancelled for subscription: %s", subscriptionID.Hex())

	err := c.paymentHandler.HandlePaymentCancelled(ctx, subscriptionID, event.OrderCode)
	if err != nil {
		return fmt.Errorf("failed to handle payment cancelled: %w", err)
	}

	log.Printf("Successfully processed payment cancelled for subscription %s", subscriptionID.Hex())
	return nil
}

func (c *EventConsumer) handlePaymentTimeout(ctx context.Context, subscriptionID bson.ObjectID, event PaymentEventData) error {
	log.Printf("Handling payment timeout for subscription: %s", subscriptionID.Hex())

	err := c.paymentHandler.HandlePaymentTimeout(ctx, subscriptionID, event.OrderCode)
	if err != nil {
		return fmt.Errorf("failed to handle payment timeout: %w", err)
	}

	log.Printf("Successfully processed payment timeout for subscription %s", subscriptionID.Hex())
	return nil
}

func (c *EventConsumer) Close() error {
	if !c.enabled {
		return nil
	}

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
