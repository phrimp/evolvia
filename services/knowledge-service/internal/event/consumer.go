package event

import (
	"context"
	"encoding/json"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"knowledge-service/internal/services"
	"log"
	"math"
	"strings"
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
	skillService     *services.SkillService
	enabled          bool
}

func NewEventConsumer(rabbitURI string, userSkillService *services.UserSkillService, skillService *services.SkillService) (*EventConsumer, error) {
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
	exchangeName := "skills.events"
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
		skillService:     skillService,
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

	log.Printf("Processing input skill event for user %s from source: %s", inputEvent.UserID, inputEvent.Source)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Convert user ID to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(inputEvent.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Extract skills from the text content
	detectedSkills, err := c.detectSkillsFromText(ctx, inputEvent.Data.TextForAnalysis)
	if err != nil {
		log.Printf("Failed to detect skills from text: %v", err)
		return err
	}

	log.Printf("Detected %d skills from text for user %s", len(detectedSkills), inputEvent.UserID)

	// Process each detected skill and add to user's profile
	addedCount := 0
	for _, skillMatch := range detectedSkills {
		if err := c.addSkillToUser(ctx, userObjectID, skillMatch, inputEvent.Source); err != nil {
			log.Printf("Failed to add skill '%s' to user %s: %v", skillMatch.SkillName, inputEvent.UserID, err)
			continue
		}
		addedCount++
	}

	log.Printf("Successfully added %d skills to user %s from %s", addedCount, inputEvent.UserID, inputEvent.Source)
	return nil
}

// detectSkillsFromText analyzes text content and identifies skills
func (c *EventConsumer) detectSkillsFromText(ctx context.Context, text string) ([]*SkillMatch, error) {
	if text == "" {
		return []*SkillMatch{}, nil
	}

	// Get all active skills for matching
	skills, _, err := c.skillService.ListSkills(ctx, repository.ListOptions{
		ActiveOnly: true,
		Limit:      1000, // Get more skills for better matching
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get skills for matching: %w", err)
	}

	var matches []*SkillMatch
	textLower := strings.ToLower(text)

	for _, skill := range skills {
		match := c.matchSkillInText(skill, textLower)
		if match != nil {
			matches = append(matches, match)
		}
	}

	return matches, nil
}

// matchSkillInText checks if a skill is mentioned in the text
func (c *EventConsumer) matchSkillInText(skill *models.Skill, textLower string) *SkillMatch {
	rawScore := 0.0
	var bestMatch string

	// Check primary patterns
	for _, pattern := range skill.IdentificationRules.PrimaryPatterns {
		if c.textContainsPattern(textLower, pattern) {
			rawScore += pattern.Weight * 0.8 // Primary patterns have high weight
			if bestMatch == "" {
				bestMatch = pattern.Text
			}
		}
	}

	// Check secondary patterns
	for _, pattern := range skill.IdentificationRules.SecondaryPatterns {
		if c.textContainsPattern(textLower, pattern) {
			rawScore += pattern.Weight * 0.5 // Secondary patterns have medium weight
		}
	}

	// Check skill name and common names
	if strings.Contains(textLower, strings.ToLower(skill.Name)) {
		rawScore += 0.7
		if bestMatch == "" {
			bestMatch = skill.Name
		}
	}

	for _, commonName := range skill.CommonNames {
		if strings.Contains(textLower, strings.ToLower(commonName)) {
			rawScore += 0.6
			if bestMatch == "" {
				bestMatch = commonName
			}
		}
	}

	// Check abbreviations and technical terms
	for _, abbrev := range skill.Abbreviations {
		if strings.Contains(textLower, strings.ToLower(abbrev)) {
			rawScore += 0.5
		}
	}

	for _, term := range skill.TechnicalTerms {
		if strings.Contains(textLower, strings.ToLower(term)) {
			rawScore += 0.4
		}
	}

	// Apply minimum confidence threshold
	minConfidence := skill.IdentificationRules.MinTotalScore
	if minConfidence == 0 {
		minConfidence = 0.3 // Default minimum confidence
	}

	// Normalize confidence to be between 0 and 1
	// Use a sigmoid-like function to map raw scores to [0, 1]
	normalizedConfidence := c.normalizeConfidence(rawScore)

	if normalizedConfidence >= minConfidence {
		return &SkillMatch{
			SkillID:     skill.ID,
			SkillName:   skill.Name,
			Confidence:  normalizedConfidence,
			MatchedText: bestMatch,
		}
	}

	return nil
}

// normalizeConfidence converts raw score to a value between 0 and 1
func (c *EventConsumer) normalizeConfidence(rawScore float64) float64 {
	// Use a tanh-based normalization to map [0, infinity] to [0, 1]
	// This prevents confidence from exceeding 1.0
	if rawScore <= 0 {
		return 0
	}

	// Scale the raw score and apply tanh normalization
	scaledScore := rawScore / 2.0 // Adjust scaling factor as needed
	confidence := math.Tanh(scaledScore)

	// Ensure minimum confidence for any detection
	if confidence < 0.1 {
		confidence = 0.1
	}

	// Ensure maximum confidence doesn't exceed 0.95
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// textContainsPattern checks if text contains a specific pattern
func (c *EventConsumer) textContainsPattern(text string, pattern models.KeywordPattern) bool {
	patternText := strings.ToLower(pattern.Text)
	if pattern.CaseSensitive {
		// For case-sensitive patterns, use original case
		return strings.Contains(text, pattern.Text)
	}
	return strings.Contains(text, patternText)
}

// addSkillToUser adds a detected skill to user's profile
func (c *EventConsumer) addSkillToUser(ctx context.Context, userID bson.ObjectID, skillMatch *SkillMatch, source string) error {
	// Check if user already has this skill
	existing, err := c.userSkillService.GetUserSkill(ctx, userID, skillMatch.SkillID)
	if err == nil && existing != nil {
		// User already has this skill, update confidence if higher
		if skillMatch.Confidence > existing.Confidence {
			updates := &services.UserSkillUpdate{
				Confidence: &skillMatch.Confidence,
			}
			now := time.Now()
			updates.LastUsed = &now

			_, err := c.userSkillService.UpdateUserSkill(ctx, userID, skillMatch.SkillID, updates)
			if err != nil {
				return fmt.Errorf("failed to update existing user skill: %w", err)
			}
			log.Printf("Updated confidence for skill '%s' for user %s", skillMatch.SkillName, userID.Hex())
		}
		return nil
	}

	// Determine skill level based on confidence
	level := c.determineSkillLevel(skillMatch.Confidence)

	// Create new user skill
	userSkill := &models.UserSkill{
		UserID:          userID,
		SkillID:         skillMatch.SkillID,
		Level:           level,
		Confidence:      skillMatch.Confidence,
		YearsExperience: 0, // Default, can be improved with more analysis
		Verified:        false,
		Endorsements:    0,
	}

	_, err = c.userSkillService.AddUserSkill(ctx, userSkill)
	if err != nil {
		return fmt.Errorf("failed to add user skill: %w", err)
	}

	log.Printf("Added skill '%s' (level: %s, confidence: %.2f) to user %s from source: %s",
		skillMatch.SkillName, level, skillMatch.Confidence, userID.Hex(), source)
	return nil
}

// determineSkillLevel maps confidence to skill level
func (c *EventConsumer) determineSkillLevel(confidence float64) models.SkillLevel {
	switch {
	case confidence >= 0.8:
		return models.SkillLevelAdvanced
	case confidence >= 0.6:
		return models.SkillLevelIntermediate
	case confidence >= 0.4:
		return models.SkillLevelBeginner
	default:
		return models.SkillLevelBeginner
	}
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

// SkillMatch represents a skill detected in text
type SkillMatch struct {
	SkillID     bson.ObjectID `json:"skill_id"`
	SkillName   string        `json:"skill_name"`
	Confidence  float64       `json:"confidence"`
	MatchedText string        `json:"matched_text"`
}
