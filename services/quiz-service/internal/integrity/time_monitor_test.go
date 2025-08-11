package integrity

import (
	"quiz-service/internal/models"
	"testing"
)

func TestTimeIntegrityMonitor_ValidateQuestionTiming(t *testing.T) {
	testCases := []struct {
		name             string
		actualTime       int
		isCorrect        bool
		expectedCount    int
		expectedSeverity string
		description      string
	}{
		{
			name:          "Normal timing",
			actualTime:    45,
			isCorrect:     true,
			expectedCount: 0,
			description:   "Should pass with no violations",
		},
		{
			name:             "Too fast - critical violation",
			actualTime:       2,
			isCorrect:        true,
			expectedCount:    1,
			expectedSeverity: "high",
			description:      "Should detect too-fast answer",
		},
		{
			name:             "Suspicious timing - 3x estimated",
			actualTime:       180, // 3 minutes (3x estimated)
			isCorrect:        true,
			expectedCount:    1,
			expectedSeverity: "medium",
			description:      "Should detect suspicious timing",
		},
		{
			name:             "Potential cheating - 5x estimated",
			actualTime:       300, // 5 minutes (5x estimated)
			isCorrect:        true,
			expectedCount:    1,
			expectedSeverity: "critical",
			description:      "Should detect potential cheating",
		},
		{
			name:          "Exceeds maximum time",
			actualTime:    2000, // Over 30 minutes
			isCorrect:     false,
			expectedCount: 2, // Both exceeds_maximum and potential_cheating
			description:   "Should detect maximum time violation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh monitor for each test to avoid pattern detection interference
			monitor := NewTimeIntegrityMonitor(nil)

			question := &models.Question{
				ID:                   "q1",
				Content:              "Test question",
				EstimatedTimeSeconds: 60, // 1 minute estimated
				BloomLevel:           "apply",
				DifficultyLevel:      "medium",
			}

			// Use unique session ID for each test case
			sessionID := "test_session_" + tc.name

			violations := monitor.ValidateQuestionTiming(
				sessionID,
				question,
				tc.actualTime,
				tc.isCorrect,
			)

			if len(violations) != tc.expectedCount {
				t.Errorf("Expected %d violations, got %d. Violations: %+v", tc.expectedCount, len(violations), violations)
				for i, v := range violations {
					t.Logf("Violation %d: Type=%s, Severity=%s", i, v.Type, v.Severity)
				}
			}

			if tc.expectedCount > 0 && tc.expectedSeverity != "" {
				found := false
				for _, violation := range violations {
					if violation.Severity == tc.expectedSeverity {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected violation with severity %s", tc.expectedSeverity)
				}
			}
		})
	}
}

func TestTimeIntegrityMonitor_PatternDetection(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	question := &models.Question{
		ID:                   "q1",
		Content:              "Test question",
		EstimatedTimeSeconds: 30,
		BloomLevel:           "remember",
		DifficultyLevel:      "easy",
	}

	sessionID := "pattern_test_session"

	// Create a pattern of too-fast answers (auto-clicker simulation)
	for i := 0; i < 5; i++ {
		violations := monitor.ValidateQuestionTiming(sessionID, question, 2, true)

		// The last few should trigger pattern detection
		if i >= 2 {
			patternDetected := false
			for _, violation := range violations {
				if violation.Type == "pattern_too_fast" {
					patternDetected = true
					break
				}
			}
			// Pattern should be detected by the 3rd consecutive fast answer
			if i >= 3 && !patternDetected {
				t.Errorf("Expected pattern detection on iteration %d", i+1)
			}
		}
	}
}

func TestSessionIntegrityReport(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	question1 := &models.Question{
		ID: "q1", EstimatedTimeSeconds: 30, BloomLevel: "remember", DifficultyLevel: "easy",
	}
	question2 := &models.Question{
		ID: "q2", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium",
	}

	sessionID := "report_test_session"

	// Normal answer
	monitor.ValidateQuestionTiming(sessionID, question1, 25, true)

	// Suspicious answer
	monitor.ValidateQuestionTiming(sessionID, question2, 200, true) // 3x+ estimated

	// Too fast answer
	monitor.ValidateQuestionTiming(sessionID, question1, 3, false)

	report := monitor.GetSessionIntegrityReport(sessionID)

	if report.TotalQuestions != 3 {
		t.Errorf("Expected 3 questions, got %d", report.TotalQuestions)
	}

	if report.TotalViolations == 0 {
		t.Errorf("Expected violations to be detected")
	}

	if report.IntegrityScore >= 100 {
		t.Errorf("Expected integrity score to be reduced due to violations, got %.2f", report.IntegrityScore)
	}

	if report.RiskLevel == "none" {
		t.Errorf("Expected risk level to be elevated due to violations, got %s", report.RiskLevel)
	}

	// Test different risk levels
	if report.IntegrityScore < 50 && report.RiskLevel != "critical" {
		t.Errorf("Expected critical risk level for score %.2f", report.IntegrityScore)
	}
}

func TestAlternatingPatternDetection(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	question := &models.Question{
		ID: "q_alt", EstimatedTimeSeconds: 60, BloomLevel: "analyze", DifficultyLevel: "hard",
	}

	sessionID := "alternating_test"

	// Create alternating fast/slow pattern (possible external assistance)
	timings := []int{10, 120, 8, 150, 12, 140} // Fast, slow, fast, slow, fast, slow

	for i, timing := range timings {
		violations := monitor.ValidateQuestionTiming(sessionID, question, timing, true)

		if i >= 4 { // After 5 answers, pattern should be detected
			patternDetected := false
			for _, violation := range violations {
				if violation.Type == "pattern_alternating" {
					patternDetected = true
					t.Logf("Alternating pattern detected after %d questions", i+1)
					break
				}
			}
			if i == len(timings)-1 && !patternDetected {
				t.Errorf("Expected alternating pattern to be detected")
			}
		}
	}
}

func TestConsistentTimingPattern(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	question := &models.Question{
		ID: "q_consistent", EstimatedTimeSeconds: 45, BloomLevel: "understand", DifficultyLevel: "easy",
	}

	sessionID := "consistent_test"

	// Create unnaturally consistent timing (possible automation)
	baseTime := 47
	for i := 0; i < 6; i++ {
		// Very slight variation (within 25% threshold)
		timing := baseTime + (i % 3) // 47, 48, 49, 47, 48, 49
		violations := monitor.ValidateQuestionTiming(sessionID, question, timing, true)

		if i >= 4 { // After 5 answers
			patternDetected := false
			for _, violation := range violations {
				if violation.Type == "pattern_too_consistent" {
					patternDetected = true
					t.Logf("Consistent pattern detected after %d questions", i+1)
					break
				}
			}
			if i == 5 && !patternDetected {
				t.Errorf("Expected consistent timing pattern to be detected")
			}
		}
	}
}

func TestCustomConfig(t *testing.T) {
	config := &TimeIntegrityConfig{
		SuspiciousTimeMultiplier: 2.0,   // Lower threshold
		CheatingTimeMultiplier:   3.0,   // Lower threshold
		MinimumQuestionTime:      10,    // Higher minimum
		MaxQuestionTime:          300,   // Lower maximum
		EnablePatternDetection:   false, // Disable patterns
		PatternWindowSize:        3,
	}

	monitor := NewTimeIntegrityMonitor(config)

	question := &models.Question{
		ID: "q_custom", EstimatedTimeSeconds: 30, BloomLevel: "apply", DifficultyLevel: "medium",
	}

	// Test with custom thresholds
	violations := monitor.ValidateQuestionTiming("custom_session", question, 8, true)

	// Should violate minimum time (10 seconds)
	found := false
	for _, violation := range violations {
		if violation.Type == "too_fast" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected minimum time violation with custom config")
	}

	// Test suspicious timing with lower threshold (2x instead of 3x)
	violations2 := monitor.ValidateQuestionTiming("custom_session", question, 65, true) // 2.17x estimated

	found = false
	for _, violation := range violations2 {
		if violation.Type == "suspicious_timing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected suspicious timing violation with custom lower threshold")
	}
}

