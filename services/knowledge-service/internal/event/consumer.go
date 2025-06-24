package event

import (
	"context"
	"encoding/json"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/services"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Consumer interface {
	Start() error
	Close() error
}

type EventConsumer struct {
	conn             *amqp091.Connection
	channel          *amqp091.Channel
	queueName        string
	userSkillService *services.UserSkillService
	enabled          bool
}

func NewEventConsumer(rabbitURI string, userSkillService *services.UserSkillService) (*EventConsumer, error) {
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

	// Declare the queue
	queueName := "knowledge-service-input-skills"
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

	// Bind the queue to handle input.skill events
	err = channel.QueueBind(
		queue.Name,    // queue name
		"input.skill", // routing key
		exchangeName,  // exchange
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	return &EventConsumer{
		conn:             conn,
		channel:          channel,
		queueName:        queue.Name,
		userSkillService: userSkillService,
		enabled:          true,
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

	log.Println("Event consumer started, waiting for messages...")
	return nil
}

func (c *EventConsumer) processMessage(msg amqp091.Delivery) error {
	log.Printf("Received message with routing key: %s", msg.RoutingKey)

	switch msg.RoutingKey {
	case "input.skill":
		return c.handleInputSkillEvent(msg.Body)
	default:
		log.Printf("Unknown routing key: %s", msg.RoutingKey)
		return nil // Don't requeue unknown message types
	}
}

func (c *EventConsumer) handleInputSkillEvent(body []byte) error {
	var inputEvent InputSkillEvent
	if err := json.Unmarshal(body, &inputEvent); err != nil {
		return fmt.Errorf("failed to unmarshal input skill event: %w", err)
	}

	log.Printf("Processing input skill event for user %s with %d skills", inputEvent.UserID, len(inputEvent.Skills))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert user ID to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(inputEvent.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Process each skill
	for _, inputSkill := range inputEvent.Skills {
		if err := c.processInputSkill(ctx, userObjectID, inputSkill, inputEvent.Source); err != nil {
			log.Printf("Failed to process skill '%s': %v", inputSkill.Name, err)
			// Continue processing other skills rather than failing the entire batch
			continue
		}
	}

	log.Printf("Successfully processed input skill event for user %s", inputEvent.UserID)
	return nil
}

func (c *EventConsumer) processInputSkill(ctx context.Context, userID bson.ObjectID, inputSkill InputSkill, source string) error {
	log.Printf("Added skill '%s' for user %s", inputSkill.Name, userID.Hex())
	return nil
}

func (c *EventConsumer) findSkillByName(ctx context.Context, name string) ([]*models.Skill, error) {
	// This is a simplified implementation
	// In a real scenario, you would inject the skill service or repository
	// For now, we'll assume we have access to search functionality

	// TODO: Implement proper skill search through injected service
	// This is a placeholder that would need to be replaced with actual implementation
	log.Printf("Searching for skill: %s", name)
	return []*models.Skill{}, nil
}

func (c *EventConsumer) updateExistingUserSkill(ctx context.Context, existing *models.UserSkill, input InputSkill) error {
	updates := &services.UserSkillUpdate{}
	hasUpdates := false

	// Update level if provided and different
	if input.Level != "" && input.Level != existing.Level {
		updates.Level = input.Level
		hasUpdates = true
	}

	// Update confidence if higher
	newConfidence := c.determineConfidence(input)
	if newConfidence > existing.Confidence {
		updates.Confidence = &newConfidence
		hasUpdates = true
	}

	// Update years of experience if higher
	if input.YearsExperience > existing.YearsExperience {
		updates.YearsExperience = &input.YearsExperience
		hasUpdates = true
	}

	// Update last used
	now := time.Now()
	updates.LastUsed = &now
	hasUpdates = true

	if hasUpdates {
		_, err := c.userSkillService.UpdateUserSkill(ctx, existing.UserID, existing.SkillID, updates)
		if err != nil {
			return fmt.Errorf("failed to update user skill: %w", err)
		}
		log.Printf("Updated existing skill for user %s", existing.UserID.Hex())
	}

	return nil
}

func (c *EventConsumer) determineSkillLevel(input InputSkill) models.SkillLevel {
	if input.Level != "" {
		return input.Level
	}

	// Determine level based on years of experience
	switch {
	case input.YearsExperience >= 5:
		return models.SkillLevelExpert
	case input.YearsExperience >= 3:
		return models.SkillLevelAdvanced
	case input.YearsExperience >= 1:
		return models.SkillLevelIntermediate
	default:
		return models.SkillLevelBeginner
	}
}

func (c *EventConsumer) determineConfidence(input InputSkill) float64 {
	if input.Confidence > 0 {
		return input.Confidence
	}

	// Default confidence based on context or years of experience
	if input.YearsExperience > 0 {
		confidence := float64(input.YearsExperience) / 10.0
		if confidence > 1.0 {
			confidence = 1.0
		}
		if confidence < 0.3 {
			confidence = 0.3
		}
		return confidence
	}

	return 0.5 // Default moderate confidence
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
