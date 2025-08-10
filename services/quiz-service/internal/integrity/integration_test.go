package integrity

import (
	"quiz-service/internal/models"
	"testing"
	"time"
)

func TestIntegrityMonitorIntegration(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)
	sessionID := "integration_test_session"
	
	// Create test questions with different estimated times
	questions := []*models.Question{
		{
			ID:                   "q1",
			Content:              "Easy question",
			EstimatedTimeSeconds: 30,
			BloomLevel:           "remember",
			DifficultyLevel:      "easy",
		},
		{
			ID:                   "q2", 
			Content:              "Medium question",
			EstimatedTimeSeconds: 60,
			BloomLevel:           "apply",
			DifficultyLevel:      "medium",
		},
		{
			ID:                   "q3",
			Content:              "Hard question", 
			EstimatedTimeSeconds: 120,
			BloomLevel:           "analyze",
			DifficultyLevel:      "hard",
		},
	}
	
	// Test scenario: Normal -> Suspicious -> Critical progression
	testScenarios := []struct {
		question      *models.Question
		timeSpent     int
		isCorrect     bool
		expectedRisk  string
		description   string
	}{
		{questions[0], 25, true, "none", "Normal timing for easy question"},
		{questions[1], 45, true, "none", "Normal timing for medium question"},
		{questions[2], 200, true, "low", "Suspicious timing - 1.67x estimated"},
		{questions[1], 250, false, "medium", "Very suspicious - 4.2x estimated"},
		{questions[0], 2, true, "high", "Too fast answer - below minimum"},
	}
	
	for i, scenario := range testScenarios {
		t.Run(scenario.description, func(t *testing.T) {
			violations := monitor.ValidateQuestionTiming(
				sessionID,
				scenario.question,
				scenario.timeSpent,
				scenario.isCorrect,
			)
			
			// Check for expected violations based on timing
			if scenario.timeSpent < 5 {
				// Should have "too_fast" violation
				found := false
				for _, v := range violations {
					if v.Type == "too_fast" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Step %d: Expected 'too_fast' violation for %d seconds", i+1, scenario.timeSpent)
				}
			}
			
			if float64(scenario.timeSpent)/float64(scenario.question.EstimatedTimeSeconds) >= 5.0 {
				// Should have "potential_cheating" violation
				found := false
				for _, v := range violations {
					if v.Type == "potential_cheating" && v.Severity == "critical" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Step %d: Expected 'potential_cheating' violation for ratio %.1fx", 
						i+1, float64(scenario.timeSpent)/float64(scenario.question.EstimatedTimeSeconds))
				}
			}
		})
	}
	
	// Get comprehensive integrity report
	t.Run("comprehensive_integrity_report", func(t *testing.T) {
		report := monitor.GetSessionIntegrityReport(sessionID)
		
		if report.TotalQuestions != len(testScenarios) {
			t.Errorf("Expected %d questions in report, got %d", len(testScenarios), report.TotalQuestions)
		}
		
		if report.TotalViolations == 0 {
			t.Errorf("Expected violations to be recorded, got 0")
		}
		
		if report.IntegrityScore >= 100 {
			t.Errorf("Expected integrity score to be reduced due to violations, got %.2f", report.IntegrityScore)
		}
		
		if report.RiskLevel == "none" || report.RiskLevel == "" {
			t.Errorf("Expected elevated risk level, got '%s'", report.RiskLevel)
		}
		
		// Verify violations are properly categorized
		criticalCount := 0
		highCount := 0
		for _, violation := range report.Violations {
			switch violation.Severity {
			case "critical":
				criticalCount++
			case "high":
				highCount++
			}
		}
		
		if criticalCount == 0 && highCount == 0 {
			t.Errorf("Expected at least one critical or high severity violation")
		}
		
		// Check timing history
		if len(report.TimingHistory) != len(testScenarios) {
			t.Errorf("Expected %d timing entries, got %d", len(testScenarios), len(report.TimingHistory))
		}
		
		// Verify average time ratio calculation
		if report.AverageTimeRatio <= 0 {
			t.Errorf("Expected positive average time ratio, got %.2f", report.AverageTimeRatio)
		}
		
		t.Logf("Integrity Report Summary:")
		t.Logf("  Questions: %d", report.TotalQuestions)
		t.Logf("  Violations: %d", report.TotalViolations)
		t.Logf("  Integrity Score: %.2f", report.IntegrityScore)
		t.Logf("  Risk Level: %s", report.RiskLevel)
		t.Logf("  Recommended Action: %s", report.RecommendedAction)
		t.Logf("  Average Time Ratio: %.2f", report.AverageTimeRatio)
	})
}

func TestPatternDetectionIntegration(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)
	sessionID := "pattern_integration_test"
	
	question := &models.Question{
		ID:                   "pattern_q",
		EstimatedTimeSeconds: 45,
		BloomLevel:           "understand",
		DifficultyLevel:      "medium",
	}
	
	t.Run("auto_clicker_pattern", func(t *testing.T) {
		// Simulate auto-clicker behavior - consistently too fast
		for i := 0; i < 6; i++ {
			violations := monitor.ValidateQuestionTiming(sessionID+"_clicker", question, 2, true)
			
			if i >= 2 { // After 3 questions, pattern should be detected
				patternDetected := false
				for _, v := range violations {
					if v.Type == "pattern_too_fast" {
						patternDetected = true
						t.Logf("Auto-clicker pattern detected after %d questions", i+1)
						break
					}
				}
				// Pattern should definitely be detected by question 4
				if i >= 3 && !patternDetected {
					t.Errorf("Expected auto-clicker pattern detection by question %d", i+1)
				}
			}
		}
		
		report := monitor.GetSessionIntegrityReport(sessionID + "_clicker")
		if report.RiskLevel != "critical" && report.RiskLevel != "high" {
			t.Errorf("Expected high/critical risk level for auto-clicker pattern, got %s", report.RiskLevel)
		}
	})
	
	t.Run("external_assistance_pattern", func(t *testing.T) {
		// Simulate external assistance - alternating fast/slow
		timings := []int{8, 120, 10, 150, 12, 140, 9, 160}
		
		for i, timing := range timings {
			violations := monitor.ValidateQuestionTiming(sessionID+"_assist", question, timing, true)
			
			if i >= 4 { // After 5 answers, check for pattern
				patternDetected := false
				for _, v := range violations {
					if v.Type == "pattern_alternating" {
						patternDetected = true
						t.Logf("Alternating pattern detected after %d questions", i+1)
						break
					}
				}
				// Should be detected by the end
				if i == len(timings)-1 && !patternDetected {
					t.Errorf("Expected alternating pattern to be detected")
				}
			}
		}
		
		report := monitor.GetSessionIntegrityReport(sessionID + "_assist")
		if report.RiskLevel == "none" {
			t.Errorf("Expected elevated risk level for alternating pattern, got %s", report.RiskLevel)
		}
	})
	
	t.Run("automation_pattern", func(t *testing.T) {
		// Simulate automated responses - unnaturally consistent timing
		baseTime := 47
		for i := 0; i < 6; i++ {
			// Very consistent timing (automation-like)
			timing := baseTime + (i % 2) // 47, 48, 47, 48, 47, 48
			violations := monitor.ValidateQuestionTiming(sessionID+"_auto", question, timing, true)
			
			if i >= 4 { // After 5 answers
				patternDetected := false
				for _, v := range violations {
					if v.Type == "pattern_too_consistent" {
						patternDetected = true
						t.Logf("Automation pattern detected after %d questions", i+1)
						break
					}
				}
				// Should be detected by the end
				if i == 5 && !patternDetected {
					t.Errorf("Expected automation pattern to be detected")
				}
			}
		}
		
		report := monitor.GetSessionIntegrityReport(sessionID + "_auto")
		if report.RiskLevel == "none" {
			t.Errorf("Expected elevated risk level for automation pattern, got %s", report.RiskLevel)
		}
	})
}

func TestCustomConfigIntegration(t *testing.T) {
	// Test with stricter configuration
	config := &TimeIntegrityConfig{
		SuspiciousTimeMultiplier: 2.0, // Lower threshold
		CheatingTimeMultiplier:   3.0, // Lower threshold  
		MinimumQuestionTime:      8,   // Higher minimum
		MaxQuestionTime:          300, // Lower maximum
		EnablePatternDetection:   true,
		PatternWindowSize:        3, // Smaller window
	}
	
	monitor := NewTimeIntegrityMonitor(config)
	sessionID := "strict_config_test"
	
	question := &models.Question{
		ID:                   "strict_q",
		EstimatedTimeSeconds: 60,
		BloomLevel:           "apply", 
		DifficultyLevel:      "medium",
	}
	
	testCases := []struct {
		timeSpent       int
		expectedViolation string
		description     string
	}{
		{5, "too_fast", "Should violate higher minimum time (8s)"},
		{130, "suspicious_timing", "Should violate lower suspicious threshold (2x)"},
		{190, "potential_cheating", "Should violate lower cheating threshold (3x)"},
		{350, "exceeds_maximum", "Should violate lower maximum time (300s)"},
	}
	
	for i, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			violations := monitor.ValidateQuestionTiming(sessionID, question, tc.timeSpent, true)
			
			found := false
			for _, v := range violations {
				if v.Type == tc.expectedViolation {
					found = true
					t.Logf("Step %d: Found expected violation '%s' for %d seconds", 
						i+1, tc.expectedViolation, tc.timeSpent)
					break
				}
			}
			
			if !found {
				violationTypes := make([]string, len(violations))
				for j, v := range violations {
					violationTypes[j] = v.Type
				}
				t.Errorf("Step %d: Expected violation '%s', got: %v", 
					i+1, tc.expectedViolation, violationTypes)
			}
		})
	}
	
	// Test that stricter config produces more violations
	report := monitor.GetSessionIntegrityReport(sessionID)
	if report.TotalViolations == 0 {
		t.Errorf("Expected violations with stricter config")
	}
	
	if report.IntegrityScore > 75 {
		t.Errorf("Expected lower integrity score with stricter config, got %.2f", report.IntegrityScore)
	}
}

func TestTimestampAndMetadata(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)
	sessionID := "metadata_test"
	
	question := &models.Question{
		ID:                   "meta_q",
		EstimatedTimeSeconds: 30,
		BloomLevel:           "remember",
		DifficultyLevel:      "easy",
	}
	
	startTime := time.Now()
	
	// Generate a violation to test metadata
	violations := monitor.ValidateQuestionTiming(sessionID, question, 2, false) // Too fast
	
	if len(violations) == 0 {
		t.Fatalf("Expected at least one violation")
	}
	
	violation := violations[0]
	
	// Verify violation metadata
	if violation.QuestionID != question.ID {
		t.Errorf("Expected violation question ID %s, got %s", question.ID, violation.QuestionID)
	}
	
	if violation.EstimatedTime != question.EstimatedTimeSeconds {
		t.Errorf("Expected violation estimated time %d, got %d", 
			question.EstimatedTimeSeconds, violation.EstimatedTime)
	}
	
	if violation.ActualTime != 2 {
		t.Errorf("Expected violation actual time 2, got %d", violation.ActualTime)
	}
	
	if violation.Timestamp.Before(startTime) {
		t.Errorf("Expected violation timestamp after test start")
	}
	
	if violation.Type == "" {
		t.Errorf("Expected violation type to be set")
	}
	
	if violation.Severity == "" {
		t.Errorf("Expected violation severity to be set")
	}
	
	if violation.Description == "" {
		t.Errorf("Expected violation description to be set")
	}
	
	if violation.RecommendedAction == "" {
		t.Errorf("Expected violation recommended action to be set")
	}
	
	// Test violation ratio calculation
	expectedRatio := float64(2) / float64(30)
	if violation.ViolationRatio != expectedRatio {
		t.Errorf("Expected violation ratio %.4f, got %.4f", expectedRatio, violation.ViolationRatio)
	}
	
	// Test report generation timestamp
	report := monitor.GetSessionIntegrityReport(sessionID)
	if report.GeneratedAt.Before(startTime) {
		t.Errorf("Expected report generation timestamp after test start")
	}
	
	if report.SessionID != sessionID {
		t.Errorf("Expected report session ID %s, got %s", sessionID, report.SessionID)
	}
}