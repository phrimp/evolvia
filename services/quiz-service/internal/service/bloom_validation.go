package service

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"quiz-service/internal/models"
)

// BloomValidationResult represents the result of Bloom breakdown validation
type BloomValidationResult struct {
	IsValid       bool     `json:"is_valid"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	TotalAnswers  int      `json:"total_answers"`
	ValidAnswers  int      `json:"valid_answers"`
	SkippedCount  int      `json:"skipped_count"`
	ValidationID  string   `json:"validation_id"`
	ProcessedAt   time.Time `json:"processed_at"`
}

// ValidateBloomBreakdownInputs validates cached answers before processing Bloom breakdown
func (s *SessionService) ValidateBloomBreakdownInputs(sessionID string, cachedAnswers []models.CachedAnswer) *BloomValidationResult {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BLOOM_VALIDATION] [%s] PANIC RECOVERED during validation: %v", sessionID, r)
		}
	}()

	validationID := fmt.Sprintf("bloom_validation_%d", time.Now().UnixNano())
	log.Printf("[BLOOM_VALIDATION] [%s] [%s] Starting validation of %d cached answers", sessionID, validationID, len(cachedAnswers))

	result := &BloomValidationResult{
		IsValid:      true,
		Errors:       []string{},
		Warnings:     []string{},
		TotalAnswers: len(cachedAnswers),
		ValidAnswers: 0,
		SkippedCount: 0,
		ValidationID: validationID,
		ProcessedAt:  time.Now(),
	}

	if len(cachedAnswers) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "No cached answers provided for validation")
		log.Printf("[BLOOM_VALIDATION] [%s] [%s] ERROR: No cached answers to validate", sessionID, validationID)
		return result
	}

	validBloomLevels := map[string]bool{
		"remember":   true,
		"understand": true,
		"apply":      true,
		"analyze":    true,
		"evaluate":   true,
		"create":     true,
	}

	bloomLevelCounts := make(map[string]int)
	var totalTimeSpent int
	var totalPoints float64

	for i, answer := range cachedAnswers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[BLOOM_VALIDATION] [%s] [%s] PANIC validating answer %d: %v", sessionID, validationID, i, r)
					result.Errors = append(result.Errors, fmt.Sprintf("Answer %d validation failed with panic: %v", i, r))
				}
			}()

			// Validate QuestionID
			if answer.QuestionID == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("Answer %d has empty QuestionID", i))
				result.SkippedCount++
				return
			}

			// Validate BloomLevel
			if answer.BloomLevel == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Answer %d (QuestionID: %s) has empty BloomLevel", i, answer.QuestionID))
				result.SkippedCount++
				return
			}

			bloomLevel := strings.ToLower(answer.BloomLevel)
			if !validBloomLevels[bloomLevel] {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Answer %d (QuestionID: %s) has invalid BloomLevel: %s", i, answer.QuestionID, answer.BloomLevel))
				result.SkippedCount++
				return
			}

			// Validate numeric values
			if math.IsInf(answer.PointsEarned, 0) || math.IsNaN(answer.PointsEarned) {
				result.Errors = append(result.Errors, fmt.Sprintf("Answer %d (QuestionID: %s) has invalid PointsEarned: %v", i, answer.QuestionID, answer.PointsEarned))
				result.SkippedCount++
				return
			}

			if answer.TimeSpentSeconds < 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Answer %d (QuestionID: %s) has negative TimeSpentSeconds: %d", i, answer.QuestionID, answer.TimeSpentSeconds))
				// Don't skip, just warn
			}

			if answer.TimeSpentSeconds > 3600 { // More than 1 hour
				result.Warnings = append(result.Warnings, fmt.Sprintf("Answer %d (QuestionID: %s) has unusually high TimeSpentSeconds: %d", i, answer.QuestionID, answer.TimeSpentSeconds))
			}

			// Validate AnsweredAt timestamp
			if answer.AnsweredAt.IsZero() {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Answer %d (QuestionID: %s) has zero AnsweredAt timestamp", i, answer.QuestionID))
			}

			// Track valid answer
			result.ValidAnswers++
			bloomLevelCounts[bloomLevel]++
			totalTimeSpent += answer.TimeSpentSeconds
			totalPoints += answer.PointsEarned

			log.Printf("[BLOOM_VALIDATION] [%s] [%s] Answer %d validated: QuestionID=%s, BloomLevel=%s, Correct=%v, Points=%.2f", 
				sessionID, validationID, i, answer.QuestionID, bloomLevel, answer.IsCorrect, answer.PointsEarned)
		}()
	}

	// Check for distribution concerns
	if result.ValidAnswers > 0 {
		for level := range validBloomLevels {
			if bloomLevelCounts[level] == 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("No answers found for Bloom level: %s", level))
			}
		}

		avgTimePerQuestion := float64(totalTimeSpent) / float64(result.ValidAnswers)
		if avgTimePerQuestion < 5 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Average time per question seems low: %.2f seconds", avgTimePerQuestion))
		}
		if avgTimePerQuestion > 300 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Average time per question seems high: %.2f seconds", avgTimePerQuestion))
		}

		avgPointsPerQuestion := totalPoints / float64(result.ValidAnswers)
		log.Printf("[BLOOM_VALIDATION] [%s] [%s] Statistics: AvgTime=%.2fs, AvgPoints=%.2f, Distribution=%+v", 
			sessionID, validationID, avgTimePerQuestion, avgPointsPerQuestion, bloomLevelCounts)
	}

	// Determine overall validity
	if len(result.Errors) > 0 {
		result.IsValid = false
	}

	if result.ValidAnswers == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "No valid answers found after validation")
	}

	log.Printf("[BLOOM_VALIDATION] [%s] [%s] Validation completed: Valid=%v, ValidAnswers=%d/%d, Errors=%d, Warnings=%d", 
		sessionID, validationID, result.IsValid, result.ValidAnswers, result.TotalAnswers, len(result.Errors), len(result.Warnings))

	return result
}

// ValidateBloomBreakdownResult validates the calculated Bloom breakdown for consistency
func (s *SessionService) ValidateBloomBreakdownResult(sessionID string, breakdown *models.BloomBreakdown, inputValidation *BloomValidationResult) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BLOOM_RESULT_VALIDATION] [%s] PANIC RECOVERED: %v", sessionID, r)
		}
	}()

	log.Printf("[BLOOM_RESULT_VALIDATION] [%s] Starting breakdown result validation", sessionID)

	if breakdown == nil {
		return fmt.Errorf("breakdown is nil")
	}

	levels := []struct {
		name string
		perf *models.BloomLevelPerformance
	}{
		{"remember", &breakdown.Remember},
		{"understand", &breakdown.Understand},
		{"apply", &breakdown.Apply},
		{"analyze", &breakdown.Analyze},
		{"evaluate", &breakdown.Evaluate},
		{"create", &breakdown.Create},
	}

	var totalCalculatedAnswers int
	for _, level := range levels {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[BLOOM_RESULT_VALIDATION] [%s] PANIC validating level %s: %v", sessionID, level.name, r)
				}
			}()

			perf := level.perf

			// Validate numeric values
			if math.IsInf(perf.AccuracyPercentage, 0) || math.IsNaN(perf.AccuracyPercentage) {
				log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Invalid AccuracyPercentage for %s: %v", sessionID, level.name, perf.AccuracyPercentage)
			}

			if math.IsInf(perf.ScorePercentage, 0) || math.IsNaN(perf.ScorePercentage) {
				log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Invalid ScorePercentage for %s: %v", sessionID, level.name, perf.ScorePercentage)
			}

			if math.IsInf(perf.AverageQuestionScore, 0) || math.IsNaN(perf.AverageQuestionScore) {
				log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Invalid AverageQuestionScore for %s: %v", sessionID, level.name, perf.AverageQuestionScore)
			}

			if math.IsInf(perf.AverageTimePerQ, 0) || math.IsNaN(perf.AverageTimePerQ) {
				log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Invalid AverageTimePerQ for %s: %v", sessionID, level.name, perf.AverageTimePerQ)
			}

			// Validate logical consistency
			if perf.QuestionsCorrect > perf.QuestionsAttempted {
				log.Printf("[BLOOM_RESULT_VALIDATION] [%s] ERROR: More correct than attempted for %s: %d > %d", 
					sessionID, level.name, perf.QuestionsCorrect, perf.QuestionsAttempted)
			}

			if perf.QuestionsAttempted > 0 {
				expectedAccuracy := float64(perf.QuestionsCorrect) / float64(perf.QuestionsAttempted) * 100
				if math.Abs(perf.AccuracyPercentage-expectedAccuracy) > 0.01 {
					log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Accuracy mismatch for %s: calculated=%.2f, expected=%.2f", 
						sessionID, level.name, perf.AccuracyPercentage, expectedAccuracy)
				}
			}

			totalCalculatedAnswers += perf.QuestionsAttempted

			log.Printf("[BLOOM_RESULT_VALIDATION] [%s] Level %s validated: Attempted=%d, Correct=%d, Accuracy=%.2f%%, Score=%.2f/%.2f", 
				sessionID, level.name, perf.QuestionsAttempted, perf.QuestionsCorrect, perf.AccuracyPercentage, perf.ActualScore, perf.PossibleScore)
		}()
	}

	// Cross-check with input validation
	if inputValidation != nil && totalCalculatedAnswers != inputValidation.ValidAnswers {
		log.Printf("[BLOOM_RESULT_VALIDATION] [%s] WARNING: Total calculated answers (%d) != valid input answers (%d)", 
			sessionID, totalCalculatedAnswers, inputValidation.ValidAnswers)
	}

	log.Printf("[BLOOM_RESULT_VALIDATION] [%s] Breakdown validation completed successfully", sessionID)
	return nil
}