package integrity

import (
	"fmt"
	"quiz-service/internal/models"
	"time"
)

// TimeIntegrityConfig defines thresholds for time-based integrity monitoring
type TimeIntegrityConfig struct {
	// Multipliers for estimated time to determine suspicious behavior
	SuspiciousTimeMultiplier float64 // e.g., 3.0 means 3x estimated time is suspicious
	CheatingTimeMultiplier   float64 // e.g., 5.0 means 5x estimated time indicates cheating

	// Minimum time thresholds (in seconds)
	MinimumQuestionTime int // Minimum time to prevent instant answers

	// Maximum absolute time limits (in seconds)
	MaxQuestionTime int // Hard limit regardless of estimated time

	// Behavioral analysis
	EnablePatternDetection bool // Analyze timing patterns across questions
	PatternWindowSize      int  // Number of recent questions to analyze
}

// DefaultTimeIntegrityConfig returns default configuration
func DefaultTimeIntegrityConfig() *TimeIntegrityConfig {
	return &TimeIntegrityConfig{
		SuspiciousTimeMultiplier: 3.0,
		CheatingTimeMultiplier:   5.0,
		MinimumQuestionTime:      5,    // 5 seconds minimum
		MaxQuestionTime:          1800, // 30 minutes maximum
		EnablePatternDetection:   true,
		PatternWindowSize:        5,
	}
}

// TimeViolation represents a time-based integrity violation
type TimeViolation struct {
	Type              string    `json:"type"`
	Severity          string    `json:"severity"` // low, medium, high, critical
	QuestionID        string    `json:"question_id"`
	EstimatedTime     int       `json:"estimated_time"`
	ActualTime        int       `json:"actual_time"`
	ViolationRatio    float64   `json:"violation_ratio"` // actual/estimated
	Timestamp         time.Time `json:"timestamp"`
	Description       string    `json:"description"`
	RecommendedAction string    `json:"recommended_action"`
}

// TimeIntegrityMonitor monitors question timing for integrity violations
type TimeIntegrityMonitor struct {
	config           *TimeIntegrityConfig
	sessionTimings   map[string][]QuestionTiming // sessionID -> timing history
	violationHistory map[string][]TimeViolation  // sessionID -> violations
}

// QuestionTiming tracks timing data for a single question
type QuestionTiming struct {
	QuestionID      string
	EstimatedTime   int
	ActualTime      int
	IsCorrect       bool
	BloomLevel      string
	DifficultyLevel string
	Timestamp       time.Time
	ViolationFlags  []string
}

// NewTimeIntegrityMonitor creates a new time integrity monitor
func NewTimeIntegrityMonitor(config *TimeIntegrityConfig) *TimeIntegrityMonitor {
	if config == nil {
		config = DefaultTimeIntegrityConfig()
	}

	return &TimeIntegrityMonitor{
		config:           config,
		sessionTimings:   make(map[string][]QuestionTiming),
		violationHistory: make(map[string][]TimeViolation),
	}
}

// ValidateQuestionTiming validates timing for a single question
func (m *TimeIntegrityMonitor) ValidateQuestionTiming(
	sessionID string,
	question *models.Question,
	actualTimeSpent int,
	isCorrect bool,
) []TimeViolation {
	var violations []TimeViolation

	timing := QuestionTiming{
		QuestionID:      question.ID,
		EstimatedTime:   question.EstimatedTimeSeconds,
		ActualTime:      actualTimeSpent,
		IsCorrect:       isCorrect,
		BloomLevel:      question.BloomLevel,
		DifficultyLevel: question.DifficultyLevel,
		Timestamp:       time.Now(),
		ViolationFlags:  []string{},
	}

	// Check minimum time violation (too fast)
	if actualTimeSpent < m.config.MinimumQuestionTime {
		violation := TimeViolation{
			Type:              "too_fast",
			Severity:          "high",
			QuestionID:        question.ID,
			EstimatedTime:     question.EstimatedTimeSeconds,
			ActualTime:        actualTimeSpent,
			ViolationRatio:    float64(actualTimeSpent) / float64(question.EstimatedTimeSeconds),
			Timestamp:         time.Now(),
			Description:       fmt.Sprintf("Answer submitted in %d seconds (minimum: %d)", actualTimeSpent, m.config.MinimumQuestionTime),
			RecommendedAction: "flag_for_review",
		}
		violations = append(violations, violation)
		timing.ViolationFlags = append(timing.ViolationFlags, "too_fast")
	}

	// Check maximum time violation (absolute limit)
	if actualTimeSpent > m.config.MaxQuestionTime {
		violation := TimeViolation{
			Type:              "exceeds_maximum",
			Severity:          "medium",
			QuestionID:        question.ID,
			EstimatedTime:     question.EstimatedTimeSeconds,
			ActualTime:        actualTimeSpent,
			ViolationRatio:    float64(actualTimeSpent) / float64(question.EstimatedTimeSeconds),
			Timestamp:         time.Now(),
			Description:       fmt.Sprintf("Answer took %d seconds (maximum: %d)", actualTimeSpent, m.config.MaxQuestionTime),
			RecommendedAction: "timeout_question",
		}
		violations = append(violations, violation)
		timing.ViolationFlags = append(timing.ViolationFlags, "exceeds_maximum")
	}

	// Check estimated time multiplier violations
	if question.EstimatedTimeSeconds > 0 {
		ratio := float64(actualTimeSpent) / float64(question.EstimatedTimeSeconds)

		if ratio >= m.config.CheatingTimeMultiplier {
			violation := TimeViolation{
				Type:              "potential_cheating",
				Severity:          "critical",
				QuestionID:        question.ID,
				EstimatedTime:     question.EstimatedTimeSeconds,
				ActualTime:        actualTimeSpent,
				ViolationRatio:    ratio,
				Timestamp:         time.Now(),
				Description:       fmt.Sprintf("Answer took %.1fx estimated time (%.1f threshold)", ratio, m.config.CheatingTimeMultiplier),
				RecommendedAction: "terminate_session",
			}
			violations = append(violations, violation)
			timing.ViolationFlags = append(timing.ViolationFlags, "potential_cheating")

		} else if ratio >= m.config.SuspiciousTimeMultiplier {
			violation := TimeViolation{
				Type:              "suspicious_timing",
				Severity:          "medium",
				QuestionID:        question.ID,
				EstimatedTime:     question.EstimatedTimeSeconds,
				ActualTime:        actualTimeSpent,
				ViolationRatio:    ratio,
				Timestamp:         time.Now(),
				Description:       fmt.Sprintf("Answer took %.1fx estimated time (%.1f threshold)", ratio, m.config.SuspiciousTimeMultiplier),
				RecommendedAction: "increase_monitoring",
			}
			violations = append(violations, violation)
			timing.ViolationFlags = append(timing.ViolationFlags, "suspicious_timing")
		}
	}

	// Store timing data
	m.sessionTimings[sessionID] = append(m.sessionTimings[sessionID], timing)

	// Store violations
	if len(violations) > 0 {
		m.violationHistory[sessionID] = append(m.violationHistory[sessionID], violations...)
	}

	// Perform pattern analysis if enabled
	if m.config.EnablePatternDetection && len(m.sessionTimings[sessionID]) >= 3 {
		patternViolations := m.analyzeTimingPatterns(sessionID)
		violations = append(violations, patternViolations...)
	}

	return violations
}

// analyzeTimingPatterns detects suspicious patterns in timing behavior
func (m *TimeIntegrityMonitor) analyzeTimingPatterns(sessionID string) []TimeViolation {
	var violations []TimeViolation

	timings := m.sessionTimings[sessionID]
	if len(timings) < 3 {
		return violations
	}

	// Get recent timings for analysis
	windowSize := m.config.PatternWindowSize
	if len(timings) < windowSize {
		windowSize = len(timings)
	}

	recentTimings := timings[len(timings)-windowSize:]

	// Pattern 1: Consistently too fast (possible auto-clicker)
	fastCount := 0
	for _, timing := range recentTimings {
		if timing.ActualTime < m.config.MinimumQuestionTime {
			fastCount++
		}
	}

	if fastCount >= windowSize/2 { // More than half are too fast
		violation := TimeViolation{
			Type:              "pattern_too_fast",
			Severity:          "high",
			QuestionID:        "", // Pattern-based, not specific to one question
			Timestamp:         time.Now(),
			Description:       fmt.Sprintf("Pattern detected: %d out of %d recent questions answered too quickly", fastCount, windowSize),
			RecommendedAction: "terminate_session",
		}
		violations = append(violations, violation)
	}

	// Pattern 2: Alternating fast/slow (possible external assistance)
	if len(recentTimings) >= 4 {
		alternatingPattern := true
		for i := 1; i < len(recentTimings)-1; i++ {
			current := recentTimings[i].ActualTime
			prev := recentTimings[i-1].ActualTime
			next := recentTimings[i+1].ActualTime

			// Check if current is significantly different from neighbors
			if !((current < prev/2 && current < next/2) || (current > prev*2 && current > next*2)) {
				alternatingPattern = false
				break
			}
		}

		if alternatingPattern {
			violation := TimeViolation{
				Type:              "pattern_alternating",
				Severity:          "medium",
				QuestionID:        "",
				Timestamp:         time.Now(),
				Description:       "Suspicious alternating timing pattern detected (possible external assistance)",
				RecommendedAction: "increase_monitoring",
			}
			violations = append(violations, violation)
		}
	}

	// Pattern 3: Perfect timing consistency (unlikely human behavior)
	if len(recentTimings) >= 5 {
		avgTime := 0
		for _, timing := range recentTimings {
			avgTime += timing.ActualTime
		}
		avgTime /= len(recentTimings)

		variationCount := 0
		threshold := avgTime / 4 // 25% variation threshold

		for _, timing := range recentTimings {
			if abs(timing.ActualTime-avgTime) <= threshold {
				variationCount++
			}
		}

		if variationCount >= 4 { // Too consistent
			violation := TimeViolation{
				Type:              "pattern_too_consistent",
				Severity:          "medium",
				QuestionID:        "",
				Timestamp:         time.Now(),
				Description:       "Unnaturally consistent timing pattern detected (possible automation)",
				RecommendedAction: "increase_monitoring",
			}
			violations = append(violations, violation)
		}
	}

	return violations
}

// GetSessionIntegrityReport generates a comprehensive integrity report
func (m *TimeIntegrityMonitor) GetSessionIntegrityReport(sessionID string) *SessionIntegrityReport {
	timings := m.sessionTimings[sessionID]
	violations := m.violationHistory[sessionID]

	report := &SessionIntegrityReport{
		SessionID:       sessionID,
		TotalQuestions:  len(timings),
		TotalViolations: len(violations),
		GeneratedAt:     time.Now(),
	}

	if len(timings) == 0 {
		report.IntegrityScore = 100.0
		report.RiskLevel = "none"
		return report
	}

	// Calculate integrity metrics
	violationScore := 0.0
	criticalCount := 0
	highCount := 0
	mediumCount := 0

	for _, violation := range violations {
		switch violation.Severity {
		case "critical":
			violationScore += 25.0
			criticalCount++
		case "high":
			violationScore += 15.0
			highCount++
		case "medium":
			violationScore += 5.0
			mediumCount++
		case "low":
			violationScore += 1.0
		}
	}

	// Calculate integrity score (0-100, higher is better)
	report.IntegrityScore = max(0.0, 100.0-violationScore)

	// Determine risk level
	if criticalCount > 0 || report.IntegrityScore < 50 {
		report.RiskLevel = "critical"
		report.RecommendedAction = "terminate_session"
	} else if highCount > 0 || report.IntegrityScore < 70 {
		report.RiskLevel = "high"
		report.RecommendedAction = "flag_for_review"
	} else if mediumCount > 0 || report.IntegrityScore < 85 {
		report.RiskLevel = "medium"
		report.RecommendedAction = "increase_monitoring"
	} else {
		report.RiskLevel = "low"
		report.RecommendedAction = "continue"
	}

	// Calculate timing statistics
	totalEstimated := 0
	totalActual := 0
	for _, timing := range timings {
		totalEstimated += timing.EstimatedTime
		totalActual += timing.ActualTime
	}

	if totalEstimated > 0 {
		report.AverageTimeRatio = float64(totalActual) / float64(totalEstimated)
	}

	report.ViolationBreakdown = map[string]int{
		"critical": criticalCount,
		"high":     highCount,
		"medium":   mediumCount,
	}

	report.Violations = violations
	report.TimingHistory = timings

	return report
}

// SessionIntegrityReport contains comprehensive timing integrity analysis
type SessionIntegrityReport struct {
	SessionID          string           `json:"session_id"`
	TotalQuestions     int              `json:"total_questions"`
	TotalViolations    int              `json:"total_violations"`
	IntegrityScore     float64          `json:"integrity_score"` // 0-100
	RiskLevel          string           `json:"risk_level"`      // none, low, medium, high, critical
	RecommendedAction  string           `json:"recommended_action"`
	AverageTimeRatio   float64          `json:"average_time_ratio"` // actual/estimated
	ViolationBreakdown map[string]int   `json:"violation_breakdown"`
	Violations         []TimeViolation  `json:"violations"`
	TimingHistory      []QuestionTiming `json:"timing_history"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

// Helper functions
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

