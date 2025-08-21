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
	"runtime"
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
	conn                         *amqp091.Connection
	channel                      *amqp091.Channel
	queueName                    string
	userSkillService             *services.UserSkillService
	skillService                 *services.SkillService
	skillVerificationHistoryRepo *repository.SkillVerificationHistoryRepository
	enabled                      bool
}

func NewEventConsumer(rabbitURI string, userSkillService *services.UserSkillService, skillService *services.SkillService, skillVerificationHistoryRepo *repository.SkillVerificationHistoryRepository) (*EventConsumer, error) {
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

	// Bind the queue to handle quiz.session events
	err = channel.QueueBind(
		queue.Name,       // queue name
		"quiz_completed", // routing key
		exchangeName,     // exchange
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue to quiz.session events: %w", err)
	}

	return &EventConsumer{
		conn:                         conn,
		channel:                      channel,
		queueName:                    queue.Name,
		userSkillService:             userSkillService,
		skillService:                 skillService,
		skillVerificationHistoryRepo: skillVerificationHistoryRepo,
		enabled:                      true,
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
			maxRetries := 10
			retryCount := 0

			for retryCount <= maxRetries {
				if err := c.processMessage(msg); err != nil {
					retryCount++
					if retryCount > maxRetries {
						// Dead letter or error queue
						log.Printf("Message failed after %d retries, sending to DLQ", maxRetries)
						msg.Nack(false, false) // Don't requeue
						break
					}
					log.Printf("Failed to process message: %v, retry %d/%d", err, retryCount, maxRetries)
					time.Sleep(time.Duration(retryCount) * time.Second) // Exponential backoff
				} else {
					msg.Ack(false)
					break
				}
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
		return c.handleInputSkillEventWithRecovery(msg.Body)
	case "quiz_completed":
		return c.handleQuizResultEventWithRecovery(msg.Body)
	default:
		log.Printf("Unknown routing key: %s", msg.RoutingKey)
		return nil // Don't requeue unknown message types
	}
}

// handleQuizResultEventWithRecovery wraps the main handler with error recovery and metrics
func (c *EventConsumer) handleQuizResultEventWithRecovery(body []byte) (retErr error) {
	startTime := time.Now()

	// Enhanced panic recovery with stack trace
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			buf := make([]byte, 4096)
			stackSize := runtime.Stack(buf, false)
			stackTrace := string(buf[:stackSize])
			
			log.Printf("PANIC_RECOVERY in quiz result handler: %v", r)
			log.Printf("Stack trace:\n%s", stackTrace)
			log.Printf("Raw message body (first 500 chars): %s", string(body[:min(len(body), 500)]))
			log.Printf("Processing time before panic: %v", time.Since(startTime))
			
			// Convert panic to error for proper handling
			retErr = fmt.Errorf("panic recovered in quiz result handler: %v", r)
		}
	}()

	err := c.handleQuizResultEvent(body)

	// Log processing metrics
	processingTime := time.Since(startTime)
	if err != nil {
		log.Printf("EVENT_PROCESSING_FAILED: %v (processing_time: %v)", err, processingTime)
		c.logFailedEventContext(body, err)
	} else {
		log.Printf("EVENT_PROCESSING_SUCCESS: quiz.result.completed (processing_time: %v)", processingTime)
	}

	return err
}

// handleInputSkillEventWithRecovery wraps the input skill handler with error recovery
func (c *EventConsumer) handleInputSkillEventWithRecovery(body []byte) error {
	startTime := time.Now()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC_RECOVERY in input skill handler: %v", r)
			log.Printf("Raw message body (first 500 chars): %s", string(body[:min(len(body), 500)]))
		}
	}()

	err := c.handleInputSkillEvent(body)

	processingTime := time.Since(startTime)
	if err != nil {
		log.Printf("EVENT_PROCESSING_FAILED: %v (processing_time: %v)", err, processingTime)
	} else {
		log.Printf("EVENT_PROCESSING_SUCCESS: input.skill (processing_time: %v)", processingTime)
	}

	return err
}

// Fixed handleInputSkillEvent method for your consumer.go
func (c *EventConsumer) handleInputSkillEvent(body []byte) error {
	var inputEvent InputSkillEvent
	if err := json.Unmarshal(body, &inputEvent); err != nil {
		return fmt.Errorf("failed to unmarshal input skill event: %w", err)
	}

	log.Printf("Processing input skill event for user %s from source: %s", inputEvent.UserID, inputEvent.Source)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	userObjectID, err := bson.ObjectIDFromHex(inputEvent.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Pass the skillService dependency correctly
	//newSkillDiscovery := NewImprovedSkillDiscoveryService(c.skillService)
	//skillCandidates, err := newSkillDiscovery.DiscoverNewSkills(ctx, inputEvent.Data.TextForAnalysis, inputEvent.Source)
	//if err != nil {
	//	log.Printf("Failed to discover new skills: %v", err)
	//	// Continue with existing skill detection even if new skill discovery fails
	//} else {
	//	log.Printf("Discovered %d potential new skills", len(skillCandidates))

	//	// Auto-add high-confidence new skills (threshold: 0.85)
	//	err = newSkillDiscovery.AutoAddNewSkills(ctx, skillCandidates, 0.85)
	//	if err != nil {
	//		log.Printf("Failed to auto-add new skills: %v", err)
	//	}

	//	// Log candidates for manual review (lower confidence 0.7-0.84)
	//	for _, candidate := range skillCandidates {
	//		if candidate.ConfidenceScore >= 0.7 && candidate.ConfidenceScore < 0.85 {
	//			log.Printf("New skill candidate for review: %s (confidence: %.2f, frequency: %d)",
	//				candidate.Term, candidate.ConfidenceScore, candidate.Frequency)
	//		}
	//	}
	//}

	detectedSkills, err := c.detectSkillsFromText(ctx, inputEvent.Data.TextForAnalysis)
	if err != nil {
		log.Printf("Failed to detect skills from text: %v", err)
		return err
	}

	log.Printf("Detected %d existing skills from text for user %s", len(detectedSkills), inputEvent.UserID)

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
		Limit:      2000, // Get more skills for better matching
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
	if !skill.Addable {
		return nil
	}
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
		// User already has this skill - skip adding duplicate
		log.Printf("User %s already has skill '%s' - skipping duplicate", userID.Hex(), skillMatch.SkillName)
		return nil
	}

	// Create new user skill with default beginner level
	// Confidence represents how well the skill matched the content, not user proficiency
	userSkill := &models.UserSkill{
		UserID:          userID,
		SkillID:         skillMatch.SkillID,
		Level:           models.SkillLevelBeginner, // Default to beginner level
		Confidence:      skillMatch.Confidence,     // This is content-matching confidence
		YearsExperience: 0,                         // Default to 0 years
		Verified:        false,                     // Not verified initially
		Endorsements:    0,                         // No endorsements yet
	}

	_, err = c.userSkillService.AddUserSkill(ctx, userSkill)
	if err != nil {
		return fmt.Errorf("failed to add user skill: %w", err)
	}

	log.Printf("Added skill '%s' (level: %s, content-match confidence: %.2f) to user %s from source: %s",
		skillMatch.SkillName, models.SkillLevelBeginner, skillMatch.Confidence, userID.Hex(), source)
	return nil
}

// handleQuizResultEvent processes quiz completion events and creates verification history
func (c *EventConsumer) handleQuizResultEvent(body []byte) error {
	// First try parsing the consistent event structure
	var consistentEvent ConsistentQuizCompletedEvent
	if err := json.Unmarshal(body, &consistentEvent); err == nil {
		log.Printf("INFO: Successfully parsed consistent quiz completed event for user %s", consistentEvent.UserID)
		return c.processConsistentQuizEvent(&consistentEvent)
	}

	// Fallback: Parse the enriched quiz result event directly
	var quizResult QuizResultEvent
	if err := json.Unmarshal(body, &quizResult); err != nil {
		// Try parsing legacy format with nested payload structure
		var genericEvent struct {
			Type    string      `json:"type"`
			Payload interface{} `json:"payload"`
		}

		if err := json.Unmarshal(body, &genericEvent); err != nil {
			log.Printf("ERROR: Failed to unmarshal quiz result event as any known format: %s", err)
			log.Printf("Raw event data (first 500 chars): %s", string(body[:min(len(body), 500)]))
			return fmt.Errorf("failed to unmarshal quiz result event: %w", err)
		}

		// Extract the payload as QuizResultEvent
		payloadBytes, err := json.Marshal(genericEvent.Payload)
		if err != nil {
			log.Printf("failed to marshal quiz payload: %s", err)
			return fmt.Errorf("failed to marshal quiz payload: %w", err)
		}

		if err := json.Unmarshal(payloadBytes, &quizResult); err != nil {
			log.Printf("failed to unmarshal quiz result from payload: %s", err)
			log.Printf("payload content: %s", string(payloadBytes))
			return fmt.Errorf("failed to unmarshal quiz result from payload: %w", err)
		}

		// Verify that critical fields were populated from payload
		if quizResult.UserID == "" {
			log.Printf("WARNING: UserID still empty after payload parsing")
			log.Printf("Payload content: %s", string(payloadBytes))

			// Manual field extraction as fallback
			if err := c.extractCriticalFieldsFromPayload(payloadBytes, &quizResult); err != nil {
				log.Printf("Failed to extract critical fields manually: %v", err)
			}
		}
	}

	log.Printf("log quizResult event data: %v", quizResult)
	// Comprehensive event validation and logging
	if err := c.validateQuizResultEvent(&quizResult); err != nil {
		c.logEventValidationFailure(&quizResult, err)
		return fmt.Errorf("quiz result event validation failed: %w", err)
	}

	log.Printf("Processing enriched quiz completion event for user %s, session %s, config %s, score: %.2f",
		quizResult.UserID, quizResult.SessionID, quizResult.ConfigID, quizResult.FinalScore)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert user ID from string to ObjectID with enhanced error handling
	userObjectID, err := c.validateAndConvertUserID(quizResult.UserID)
	if err != nil {
		c.logUserIDConversionFailure(&quizResult, err)
		return fmt.Errorf("user ID validation failed: %w", err)
	}

	// Process skill progressions if available
	if len(quizResult.SkillProgressions) > 0 {
		log.Printf("Processing %d skill progressions for user %s", len(quizResult.SkillProgressions), quizResult.UserID)

		for _, skillProgression := range quizResult.SkillProgressions {
			err := c.processSkillProgression(ctx, userObjectID, skillProgression, &quizResult)
			if err != nil {
				log.Printf("Failed to process skill progression for skill %s: %v", skillProgression.SkillID, err)
				continue
			}
		}
	} else {
		// Fallback to legacy processing for all user skills
		log.Printf("No skill progressions found, falling back to legacy processing")
		err := c.processLegacyQuizResult(ctx, userObjectID, &quizResult)
		if err != nil {
			return fmt.Errorf("failed to process legacy quiz result: %w", err)
		}
	}

	// Log cognitive profile insights if available
	if len(quizResult.CognitiveProfile.DominantStrengths) > 0 {
		log.Printf("User %s cognitive profile - strengths: %v, growth areas: %v, complexity: %.2f",
			quizResult.UserID,
			quizResult.CognitiveProfile.DominantStrengths,
			quizResult.CognitiveProfile.GrowthAreas,
			quizResult.CognitiveProfile.CognitiveComplexity)
	}

	// Log learning patterns if available
	if quizResult.LearningPatterns.LearningStyle != "" {
		log.Printf("User %s learning patterns - style: %s, adaptability: %.2f, preferred levels: %v",
			quizResult.UserID,
			quizResult.LearningPatterns.LearningStyle,
			quizResult.LearningPatterns.AdaptabilityScore,
			quizResult.LearningPatterns.PreferredBloomLevels)
	}

	log.Printf("Successfully processed enriched quiz completion event for session %s", quizResult.SessionID)
	return nil
}

// processSkillProgression handles individual skill progression from quiz results
func (c *EventConsumer) processSkillProgression(ctx context.Context, userObjectID bson.ObjectID, skillProgression SkillProgression, quizResult *QuizResultEvent) error {
	// Convert skill ID from string to ObjectID
	skillObjectID, err := bson.ObjectIDFromHex(skillProgression.SkillID)
	if err != nil {
		return fmt.Errorf("invalid skill ID format: %w", err)
	}

	// Calculate total hours from quiz duration
	totalHours := float64(quizResult.PerformanceMetrics.DurationSeconds) / 3600.0
	if totalHours == 0 {
		// Fallback to legacy time breakdown
		totalHours = float64(quizResult.TimeBreakdown.TotalTimeSeconds) / 3600.0
	}

	// Map the Bloom improvement to assessment format
	bloomsAssessment := skillProgression.BloomImprovement
	bloomsAssessment.Verified = skillProgression.Verified
	bloomsAssessment.LastUpdated = time.Now()

	// Calculate overall score from Bloom improvement
	overallScore := bloomsAssessment.GetOverallScore()

	// Create skill progress history entry with enriched data
	progressHistory := &models.SkillProgressHistory{
		UserID:            userObjectID,
		SkillID:           skillObjectID,
		BloomsSnapshot:    bloomsAssessment,
		TotalHours:        totalHours,
		VerificationCount: 1,
		Timestamp:         quizResult.Timestamp,
		TriggerEvent:      "quiz_completion",
		OverallScore:      overallScore,
		IsAggregated:      false,
	}

	// Save to verification history repository
	_, err = c.skillVerificationHistoryRepo.Create(ctx, progressHistory)
	if err != nil {
		return fmt.Errorf("failed to create verification history: %w", err)
	}

	log.Printf("Created verification history for skill %s (%s): progress gain %.2f, overall score %.2f",
		skillProgression.SkillID, skillProgression.SkillName, skillProgression.ProgressGain, overallScore)

	return nil
}

// processLegacyQuizResult handles quiz results without skill progressions (legacy format)
func (c *EventConsumer) processLegacyQuizResult(ctx context.Context, userObjectID bson.ObjectID, quizResult *QuizResultEvent) error {
	// Get user's skills to determine which ones to update verification history for
	userSkills, err := c.userSkillService.GetUserSkills(ctx, userObjectID, repository.UserSkillListOptions{})
	if err != nil {
		log.Printf("Warning: Could not retrieve user skills for verification update: %v", err)
		return err
	}

	// Create verification history entries for each user skill
	totalHistoryEntries := 0
	for _, userSkill := range userSkills {
		// Map quiz Bloom breakdown to knowledge service Bloom assessment
		bloomsAssessment := c.mapQuizBloomToKnowledgeBloom(quizResult.BloomBreakdown)

		// Calculate estimated time spent (convert seconds to hours)
		totalHours := float64(quizResult.TimeBreakdown.TotalTimeSeconds) / 3600.0

		// Create skill progress history entry
		progressHistory := &models.SkillProgressHistory{
			UserID:            userObjectID,
			SkillID:           userSkill.SkillID,
			BloomsSnapshot:    bloomsAssessment,
			TotalHours:        totalHours,
			VerificationCount: 1,
			Timestamp:         time.Now(),
			TriggerEvent:      "quiz_verification",
			OverallScore:      bloomsAssessment.GetOverallScore(),
			IsAggregated:      false,
		}

		// Save to verification history repository
		_, err := c.skillVerificationHistoryRepo.Create(ctx, progressHistory)
		if err != nil {
			log.Printf("Failed to create verification history for user %s, skill %s: %v",
				quizResult.UserID, userSkill.SkillID.Hex(), err)
			continue
		}

		totalHistoryEntries++
		log.Printf("Created legacy verification history entry for user %s, skill %s, overall score: %.2f",
			quizResult.UserID, userSkill.SkillID.Hex(), bloomsAssessment.GetOverallScore())
	}

	log.Printf("Successfully created %d legacy verification history entries for quiz %s",
		totalHistoryEntries, quizResult.ResultID)

	return nil
}

// mapQuizBloomToKnowledgeBloom converts quiz Bloom breakdown to knowledge service format
func (c *EventConsumer) mapQuizBloomToKnowledgeBloom(quizBloom QuizBloomBreakdown) models.BloomsTaxonomyAssessment {
	// Convert accuracy percentages from quiz performance to normalized scores (0.0-1.0)
	// We use accuracy percentage as the primary indicator of competency at each level

	return models.BloomsTaxonomyAssessment{
		Remember:    c.normalizeBloomScore(quizBloom.Remember.AccuracyPercentage),
		Understand:  c.normalizeBloomScore(quizBloom.Understand.AccuracyPercentage),
		Apply:       c.normalizeBloomScore(quizBloom.Apply.AccuracyPercentage),
		Analyze:     c.normalizeBloomScore(quizBloom.Analyze.AccuracyPercentage),
		Evaluate:    c.normalizeBloomScore(quizBloom.Evaluate.AccuracyPercentage),
		Create:      c.normalizeBloomScore(quizBloom.Create.AccuracyPercentage),
		Verified:    true, // Mark as verified since it comes from quiz performance
		LastUpdated: time.Now(),
	}
}

// normalizeBloomScore converts percentage (0-100) to normalized score (0.0-1.0)
func (c *EventConsumer) normalizeBloomScore(percentage float64) float64 {
	if percentage < 0 {
		return 0.0
	}
	if percentage > 100 {
		return 1.0
	}
	return percentage / 100.0
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

// validateQuizResultEvent performs comprehensive validation of quiz result events
func (c *EventConsumer) validateQuizResultEvent(event *QuizResultEvent) error {
	// Check required fields
	if event.UserID == "" {
		return fmt.Errorf("missing required field: user_id")
	}
	if event.SessionID == "" {
		return fmt.Errorf("missing required field: session_id")
	}
	if event.ResultID == "" {
		return fmt.Errorf("missing required field: result_id")
	}

	// Validate UserID format
	if !isValidObjectIDHex(event.UserID) {
		return fmt.Errorf("invalid user_id format: '%s' is not a valid ObjectID hex string", event.UserID)
	}

	// Validate score ranges
	if event.FinalScore < 0 || event.FinalScore > 100 {
		return fmt.Errorf("invalid final_score: %.2f (must be between 0 and 100)", event.FinalScore)
	}

	// Validate skill progressions if present
	for i, progression := range event.SkillProgressions {
		if progression.SkillID == "" {
			return fmt.Errorf("skill_progressions[%d]: missing skill_id", i)
		}
		if !isValidObjectIDHex(progression.SkillID) {
			return fmt.Errorf("skill_progressions[%d]: invalid skill_id format: '%s'", i, progression.SkillID)
		}
	}

	return nil
}

// validateAndConvertUserID validates and converts user ID to ObjectID with detailed error context
func (c *EventConsumer) validateAndConvertUserID(userID string) (bson.ObjectID, error) {
	// Check for empty UserID
	if userID == "" {
		return bson.ObjectID{}, fmt.Errorf("user ID is empty")
	}

	// Check string length (ObjectID hex strings are 24 characters)
	if len(userID) != 24 {
		return bson.ObjectID{}, fmt.Errorf("user ID length invalid: got %d characters, expected 24 for ObjectID hex string", len(userID))
	}

	// Validate hex format before conversion
	if !isValidObjectIDHex(userID) {
		return bson.ObjectID{}, fmt.Errorf("user ID contains invalid hex characters: '%s'", userID)
	}

	// Convert to ObjectID
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return bson.ObjectID{}, fmt.Errorf("failed to convert user ID '%s' to ObjectID: %w", userID, err)
	}

	return objectID, nil
}

// logEventValidationFailure logs detailed information about event validation failures
func (c *EventConsumer) logEventValidationFailure(event *QuizResultEvent, validationErr error) {
	log.Printf("EVENT_VALIDATION_FAILURE: %v", validationErr)
	log.Printf("Event details - UserID: '%s' (len=%d), SessionID: '%s', ResultID: '%s', Score: %.2f",
		event.UserID, len(event.UserID), event.SessionID, event.ResultID, event.FinalScore)

	// Log skill progressions if present
	if len(event.SkillProgressions) > 0 {
		log.Printf("SkillProgressions count: %d", len(event.SkillProgressions))
		for i, prog := range event.SkillProgressions {
			log.Printf("  [%d] SkillID: '%s' (len=%d, valid_hex=%t)",
				i, prog.SkillID, len(prog.SkillID), isValidObjectIDHex(prog.SkillID))
		}
	}

	// Log event metadata
	log.Printf("Event metadata - ConfigID: '%s', TimeBreakdown: %+v",
		event.ConfigID, event.TimeBreakdown)
}

// logUserIDConversionFailure logs detailed information about UserID conversion failures
func (c *EventConsumer) logUserIDConversionFailure(event *QuizResultEvent, conversionErr error) {
	log.Printf("USER_ID_CONVERSION_FAILURE: %v", conversionErr)
	log.Printf("UserID analysis - Value: '%s', Length: %d, IsHex: %t",
		event.UserID, len(event.UserID), isValidObjectIDHex(event.UserID))

	// Character-by-character analysis for debugging
	if len(event.UserID) > 0 {
		log.Printf("UserID character analysis:")
		for i, char := range event.UserID {
			log.Printf("  [%d]: '%c' (0x%02X)", i, char, char)
			if i >= 10 { // Limit output for very long strings
				log.Printf("  ... (truncated at 10 characters)")
				break
			}
		}
	}

	// Log full event context for debugging
	log.Printf("Full event context - SessionID: '%s', ResultID: '%s'",
		event.SessionID, event.ResultID)
}

// logFailedEventContext logs comprehensive context for failed events
func (c *EventConsumer) logFailedEventContext(body []byte, err error) {
	log.Printf("FAILED_EVENT_CONTEXT: %v", err)
	log.Printf("Raw message length: %d bytes", len(body))

	log.Printf("Raw message content : %s", body)

	// Try to parse as generic JSON to understand structure
	var genericData map[string]any
	if jsonErr := json.Unmarshal(body, &genericData); jsonErr == nil {
		log.Printf("Generic JSON structure keys: %v", getMapKeys(genericData))

		// Log specific fields that might be problematic
		if userID, exists := genericData["user_id"]; exists {
			log.Printf("Raw user_id field: %v (type: %T)", userID, userID)
		}
		if payload, exists := genericData["payload"]; exists {
			if payloadMap, ok := payload.(map[string]interface{}); ok {
				if userID, exists := payloadMap["user_id"]; exists {
					log.Printf("Raw payload.user_id field: %v (type: %T)", userID, userID)
				}
			}
		}
	} else {
		log.Printf("Failed to parse as generic JSON: %v", jsonErr)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getMapKeys extracts keys from a map for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isValidObjectIDHex checks if a string is a valid ObjectID hex string
func isValidObjectIDHex(s string) bool {
	if len(s) != 24 {
		return false
	}

	// Try to convert and catch any errors
	_, err := bson.ObjectIDFromHex(s)
	return err == nil
}

// extractCriticalFieldsFromPayload manually extracts required fields from payload when struct unmarshaling fails
func (c *EventConsumer) extractCriticalFieldsFromPayload(payloadBytes []byte, quizResult *QuizResultEvent) error {
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payloadMap); err != nil {
		return fmt.Errorf("failed to unmarshal payload as map: %w", err)
	}

	log.Printf("Extracting critical fields from payload. Available keys: %v", getMapKeys(payloadMap))

	// Extract UserID
	if userID, exists := payloadMap["user_id"]; exists {
		if userIDStr, ok := userID.(string); ok && userIDStr != "" {
			log.Printf("Manually extracted user_id: '%s'", userIDStr)
			quizResult.UserID = userIDStr
		}
	}

	// Extract SessionID
	if sessionID, exists := payloadMap["session_id"]; exists {
		if sessionIDStr, ok := sessionID.(string); ok {
			log.Printf("Manually extracted session_id: '%s'", sessionIDStr)
			quizResult.SessionID = sessionIDStr
		}
	}

	// Extract ResultID
	if resultID, exists := payloadMap["result_id"]; exists {
		if resultIDStr, ok := resultID.(string); ok {
			log.Printf("Manually extracted result_id: '%s'", resultIDStr)
			quizResult.ResultID = resultIDStr
		}
	}

	// Extract QuizID
	if quizID, exists := payloadMap["quiz_id"]; exists {
		if quizIDStr, ok := quizID.(string); ok {
			log.Printf("Manually extracted quiz_id: '%s'", quizIDStr)
			quizResult.QuizID = quizIDStr
		}
	}

	// Extract ConfigID
	if configID, exists := payloadMap["config_id"]; exists {
		if configIDStr, ok := configID.(string); ok {
			log.Printf("Manually extracted config_id: '%s'", configIDStr)
			quizResult.ConfigID = configIDStr
		}
	}

	// Extract FinalScore
	if finalScore, exists := payloadMap["final_score"]; exists {
		if score, ok := finalScore.(float64); ok {
			log.Printf("Manually extracted final_score: %.2f", score)
			quizResult.FinalScore = score
		}
	}

	// Extract BadgeLevel
	if badgeLevel, exists := payloadMap["badge_level"]; exists {
		if badgeLevelStr, ok := badgeLevel.(string); ok {
			log.Printf("Manually extracted badge_level: '%s'", badgeLevelStr)
			quizResult.BadgeLevel = badgeLevelStr
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

// processConsistentQuizEvent processes the consistent quiz completed event structure
func (c *EventConsumer) processConsistentQuizEvent(event *ConsistentQuizCompletedEvent) error {
	log.Printf("INFO: Processing consistent quiz completion event for user %s, session %s, score: %.2f",
		event.UserID, event.SessionID, event.FinalScore)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Basic validation
	if event.UserID == "" {
		return fmt.Errorf("missing required field: user_id")
	}
	if len(event.UserID) != 24 {
		return fmt.Errorf("invalid user_id format: expected 24 characters, got %d", len(event.UserID))
	}
	if event.SessionID == "" {
		return fmt.Errorf("missing required field: session_id")
	}
	if event.FinalScore < 0 || event.FinalScore > 100 {
		return fmt.Errorf("invalid final_score: %.2f (must be between 0 and 100)", event.FinalScore)
	}

	// Convert user ID to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(event.UserID)
	if err != nil {
		return fmt.Errorf("failed to convert user_id to ObjectID: %w", err)
	}

	// Process skill progressions if available
	if len(event.SkillProgressions) > 0 {
		log.Printf("INFO: Processing %d skill progressions for user %s", len(event.SkillProgressions), event.UserID)
		
		processedCount := 0
		for i, progressionData := range event.SkillProgressions {
			if err := c.processSkillProgressionFromConsistentEvent(ctx, userObjectID, progressionData, event); err != nil {
				log.Printf("WARNING: Failed to process skill progression %d: %v", i, err)
				continue
			}
			processedCount++
		}
		
		log.Printf("INFO: Successfully processed %d/%d skill progressions for user %s", 
			processedCount, len(event.SkillProgressions), event.UserID)
	} else {
		// Fallback to legacy processing for all user skills
		log.Printf("INFO: No skill progressions found, falling back to legacy processing for user %s", event.UserID)
		err := c.processLegacyQuizResultFromConsistentEvent(ctx, userObjectID, event)
		if err != nil {
			return fmt.Errorf("failed to process legacy quiz result: %w", err)
		}
	}

	log.Printf("SUCCESS: Processed consistent quiz completion event for session %s", event.SessionID)
	return nil
}

// processSkillProgressionFromConsistentEvent processes a single skill progression
func (c *EventConsumer) processSkillProgressionFromConsistentEvent(ctx context.Context, userObjectID bson.ObjectID, progressionData interface{}, event *ConsistentQuizCompletedEvent) error {
	// Convert interface{} to map for processing
	progressionMap, ok := progressionData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("skill progression data is not a map")
	}

	skillIDStr, ok := progressionMap["skill_id"].(string)
	if !ok || skillIDStr == "" {
		return fmt.Errorf("missing or invalid skill_id in skill progression")
	}

	skillObjectID, err := bson.ObjectIDFromHex(skillIDStr)
	if err != nil {
		return fmt.Errorf("invalid skill_id format: %w", err)
	}

	// Create a basic bloom assessment from the consistent event data
	bloomsAssessment := c.createBasicBloomAssessment(event.BloomBreakdown)

	// Calculate total hours (assume 1 hour for simplicity)
	totalHours := 1.0

	// Create skill progress history entry
	progressHistory := &models.SkillProgressHistory{
		UserID:            userObjectID,
		SkillID:           skillObjectID,
		BloomsSnapshot:    bloomsAssessment,
		TotalHours:        totalHours,
		VerificationCount: 1,
		Timestamp:         event.Timestamp,
		TriggerEvent:      "quiz_completion",
		OverallScore:      bloomsAssessment.GetOverallScore(),
		IsAggregated:      false,
	}

	// Save to verification history repository
	_, err = c.skillVerificationHistoryRepo.Create(ctx, progressHistory)
	if err != nil {
		return fmt.Errorf("failed to create verification history: %w", err)
	}

	log.Printf("INFO: Created verification history for skill %s, overall score %.2f",
		skillIDStr, bloomsAssessment.GetOverallScore())

	return nil
}

// processLegacyQuizResultFromConsistentEvent handles consistent events without skill progressions
func (c *EventConsumer) processLegacyQuizResultFromConsistentEvent(ctx context.Context, userObjectID bson.ObjectID, event *ConsistentQuizCompletedEvent) error {
	// Get user's skills to determine which ones to update verification history for
	userSkills, err := c.userSkillService.GetUserSkills(ctx, userObjectID, repository.UserSkillListOptions{})
	if err != nil {
		log.Printf("WARNING: Could not retrieve user skills for verification update: %v", err)
		return err
	}

	// Create verification history entries for each user skill
	totalHistoryEntries := 0
	for _, userSkill := range userSkills {
		// Create basic bloom assessment from consistent event data
		bloomsAssessment := c.createBasicBloomAssessment(event.BloomBreakdown)

		// Create skill progress history entry
		progressHistory := &models.SkillProgressHistory{
			UserID:            userObjectID,
			SkillID:           userSkill.SkillID,
			BloomsSnapshot:    bloomsAssessment,
			TotalHours:        1.0, // Simplified for consistency
			VerificationCount: 1,
			Timestamp:         event.Timestamp,
			TriggerEvent:      "quiz_verification",
			OverallScore:      bloomsAssessment.GetOverallScore(),
			IsAggregated:      false,
		}

		// Save to verification history repository
		_, err := c.skillVerificationHistoryRepo.Create(ctx, progressHistory)
		if err != nil {
			log.Printf("WARNING: Failed to create verification history for user %s, skill %s: %v",
				event.UserID, userSkill.SkillID.Hex(), err)
			continue
		}

		totalHistoryEntries++
		log.Printf("INFO: Created legacy verification history entry for user %s, skill %s, overall score: %.2f",
			event.UserID, userSkill.SkillID.Hex(), bloomsAssessment.GetOverallScore())
	}

	log.Printf("SUCCESS: Created %d legacy verification history entries for quiz session %s",
		totalHistoryEntries, event.SessionID)

	return nil
}

// createBasicBloomAssessment creates a basic bloom assessment from bloom breakdown data
func (c *EventConsumer) createBasicBloomAssessment(bloomBreakdown interface{}) models.BloomsTaxonomyAssessment {
	// Default bloom assessment
	assessment := models.BloomsTaxonomyAssessment{
		Remember:    0.5,
		Understand:  0.5,
		Apply:       0.5,
		Analyze:     0.5,
		Evaluate:    0.5,
		Create:      0.5,
		Verified:    true,
		LastUpdated: time.Now(),
	}

	// Try to extract bloom data if available
	if bloomMap, ok := bloomBreakdown.(map[string]interface{}); ok {
		if remember, ok := bloomMap["remember"].(map[string]interface{}); ok {
			if accuracy, ok := remember["accuracy_percentage"].(float64); ok {
				assessment.Remember = c.normalizeBloomScore(accuracy)
			}
		}
		if understand, ok := bloomMap["understand"].(map[string]interface{}); ok {
			if accuracy, ok := understand["accuracy_percentage"].(float64); ok {
				assessment.Understand = c.normalizeBloomScore(accuracy)
			}
		}
		if apply, ok := bloomMap["apply"].(map[string]interface{}); ok {
			if accuracy, ok := apply["accuracy_percentage"].(float64); ok {
				assessment.Apply = c.normalizeBloomScore(accuracy)
			}
		}
		if analyze, ok := bloomMap["analyze"].(map[string]interface{}); ok {
			if accuracy, ok := analyze["accuracy_percentage"].(float64); ok {
				assessment.Analyze = c.normalizeBloomScore(accuracy)
			}
		}
		if evaluate, ok := bloomMap["evaluate"].(map[string]interface{}); ok {
			if accuracy, ok := evaluate["accuracy_percentage"].(float64); ok {
				assessment.Evaluate = c.normalizeBloomScore(accuracy)
			}
		}
		if create, ok := bloomMap["create"].(map[string]interface{}); ok {
			if accuracy, ok := create["accuracy_percentage"].(float64); ok {
				assessment.Create = c.normalizeBloomScore(accuracy)
			}
		}
	}

	return assessment
}
