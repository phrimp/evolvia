package integrity

import (
	"quiz-service/internal/models"
	"testing"
)

// TestHandlerIntegrationScenarios tests realistic handler integration scenarios
func TestHandlerIntegrationScenarios(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	// Scenario 1: Normal quiz session progression
	t.Run("normal_session_progression", func(t *testing.T) {
		sessionID := "normal_session"
		questions := []*models.Question{
			{ID: "q1", EstimatedTimeSeconds: 30, BloomLevel: "remember", DifficultyLevel: "easy"},
			{ID: "q2", EstimatedTimeSeconds: 45, BloomLevel: "understand", DifficultyLevel: "easy"},
			{ID: "q3", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium"},
			{ID: "q4", EstimatedTimeSeconds: 90, BloomLevel: "analyze", DifficultyLevel: "hard"},
		}

		// Normal timing progression
		timings := []int{25, 40, 55, 85}

		allViolations := []TimeViolation{}
		for i, question := range questions {
			violations := monitor.ValidateQuestionTiming(sessionID, question, timings[i], true)
			allViolations = append(allViolations, violations...)
		}

		if len(allViolations) > 0 {
			t.Errorf("Normal session should have no violations, got %d", len(allViolations))
		}

		report := monitor.GetSessionIntegrityReport(sessionID)
		if report.IntegrityScore < 90 {
			t.Errorf("Normal session should have high integrity score, got %.2f", report.IntegrityScore)
		}

		if report.RiskLevel != "none" && report.RiskLevel != "low" {
			t.Errorf("Normal session should have low/no risk, got %s", report.RiskLevel)
		}
	})

	// Scenario 2: Cheating detection progression
	t.Run("cheating_detection_progression", func(t *testing.T) {
		sessionID := "cheating_session"
		question := &models.Question{
			ID: "cheat_q", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium",
		}

		// Progressive cheating behavior: Normal -> Suspicious -> Critical
		scenarios := []struct {
			timeSpent           int
			expectedTermination bool
			stepName            string
		}{
			{50, false, "normal_start"},
			{200, false, "suspicious_timing"}, // 3.3x
			{400, true, "critical_cheating"},  // 6.7x - should terminate
		}

		for i, scenario := range scenarios {
			violations := monitor.ValidateQuestionTiming(sessionID, question, scenario.timeSpent, true)

			criticalFound := false
			for _, v := range violations {
				if v.Severity == "critical" {
					criticalFound = true
					break
				}
			}

			if scenario.expectedTermination && !criticalFound {
				t.Errorf("Step %d (%s): Expected critical violation for termination", i+1, scenario.stepName)
			}

			if !scenario.expectedTermination && criticalFound {
				t.Errorf("Step %d (%s): Unexpected critical violation", i+1, scenario.stepName)
			}
		}

		report := monitor.GetSessionIntegrityReport(sessionID)
		if report.RiskLevel != "critical" {
			t.Errorf("Cheating session should have critical risk level, got %s", report.RiskLevel)
		}

		if report.RecommendedAction != "terminate_session" {
			t.Errorf("Cheating session should recommend termination, got %s", report.RecommendedAction)
		}
	})

	// Scenario 3: Borderline suspicious behavior
	t.Run("borderline_suspicious_behavior", func(t *testing.T) {
		sessionID := "borderline_session"
		question := &models.Question{
			ID: "border_q", EstimatedTimeSeconds: 60, BloomLevel: "understand", DifficultyLevel: "medium",
		}

		// Timing just at the suspicious threshold (3x)
		timings := []int{60, 180, 75, 185, 50} // Mix of normal and 3x threshold

		violationCount := 0
		for _, timing := range timings {
			violations := monitor.ValidateQuestionTiming(sessionID, question, timing, true)
			violationCount += len(violations)
		}

		if violationCount == 0 {
			t.Errorf("Expected some violations for borderline behavior")
		}

		report := monitor.GetSessionIntegrityReport(sessionID)
		if report.RiskLevel == "none" {
			t.Errorf("Borderline behavior should have elevated risk, got %s", report.RiskLevel)
		}

		if report.RiskLevel == "critical" {
			t.Errorf("Borderline behavior should not be critical, got %s", report.RiskLevel)
		}

		// Should recommend monitoring, not termination
		if report.RecommendedAction == "terminate_session" {
			t.Errorf("Borderline behavior should not recommend termination, got %s", report.RecommendedAction)
		}
	})

	// Scenario 4: Auto-clicker simulation
	t.Run("auto_clicker_simulation", func(t *testing.T) {
		sessionID := "clicker_session"
		question := &models.Question{
			ID: "click_q", EstimatedTimeSeconds: 45, BloomLevel: "remember", DifficultyLevel: "easy",
		}

		// Consistent sub-minimum timing
		for i := 0; i < 5; i++ {
			violations := monitor.ValidateQuestionTiming(sessionID, question, 1, true)

			// Each should have individual too_fast violation
			individualViolation := false
			for _, v := range violations {
				if v.Type == "too_fast" {
					individualViolation = true
					break
				}
			}
			if !individualViolation {
				t.Errorf("Question %d: Expected individual too_fast violation", i+1)
			}

			// Pattern should be detected after multiple fast answers
			if i >= 2 {
				patternViolation := false
				for _, v := range violations {
					if v.Type == "pattern_too_fast" {
						patternViolation = true
						break
					}
				}
				if i >= 3 && !patternViolation {
					t.Errorf("Question %d: Expected pattern violation by now", i+1)
				}
			}
		}

		report := monitor.GetSessionIntegrityReport(sessionID)
		if report.RiskLevel != "critical" && report.RiskLevel != "high" {
			t.Errorf("Auto-clicker should have high/critical risk, got %s", report.RiskLevel)
		}
	})

	// Scenario 5: Mixed legitimate and suspicious behavior
	t.Run("mixed_behavior_session", func(t *testing.T) {
		sessionID := "mixed_session"
		questions := []*models.Question{
			{ID: "m1", EstimatedTimeSeconds: 30, BloomLevel: "remember", DifficultyLevel: "easy"},
			{ID: "m2", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium"},
			{ID: "m3", EstimatedTimeSeconds: 45, BloomLevel: "understand", DifficultyLevel: "easy"},
			{ID: "m4", EstimatedTimeSeconds: 90, BloomLevel: "analyze", DifficultyLevel: "hard"},
			{ID: "m5", EstimatedTimeSeconds: 40, BloomLevel: "remember", DifficultyLevel: "easy"},
		}

		// Mix of normal, suspicious, and one critical timing
		timings := []int{28, 190, 42, 85, 250} // Normal, 3x, normal, normal, 6x

		totalViolations := 0
		criticalFound := false

		for i, question := range questions {
			violations := monitor.ValidateQuestionTiming(sessionID, question, timings[i], i%2 == 0)
			totalViolations += len(violations)

			for _, v := range violations {
				if v.Severity == "critical" {
					criticalFound = true
				}
			}
		}

		if totalViolations == 0 {
			t.Errorf("Mixed session should have some violations")
		}

		if !criticalFound {
			t.Errorf("Mixed session should have at least one critical violation")
		}

		report := monitor.GetSessionIntegrityReport(sessionID)
		if report.RiskLevel == "none" {
			t.Errorf("Mixed session should have elevated risk, got %s", report.RiskLevel)
		}

		// Score should be reduced but not zero
		if report.IntegrityScore == 0 || report.IntegrityScore >= 90 {
			t.Errorf("Mixed session should have moderate integrity score, got %.2f", report.IntegrityScore)
		}
	})
}

// TestRecommendedActions validates that appropriate actions are recommended
func TestRecommendedActions(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	testCases := []struct {
		name              string
		violationSeverity string
		expectedAction    string
		description       string
	}{
		{"critical_violation", "critical", "terminate_session", "Critical violations should recommend termination"},
		{"high_violations", "high", "flag_for_review", "High violations should recommend review"},
		{"medium_violations", "medium", "increase_monitoring", "Medium violations should increase monitoring"},
		{"low_violations", "low", "continue", "Low violations should allow continuation"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionID := "action_test_" + tc.name
			question := &models.Question{
				ID: "action_q", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium",
			}

			// Create appropriate violation
			var timeSpent int
			switch tc.violationSeverity {
			case "critical":
				timeSpent = 350 // 5.8x estimated
			case "high":
				timeSpent = 2 // Below minimum
			case "medium":
				timeSpent = 200 // 3.3x estimated
			case "low":
				timeSpent = 120 // 2x estimated (should be considered normal)
			}

			monitor.ValidateQuestionTiming(sessionID, question, timeSpent, true)
			report := monitor.GetSessionIntegrityReport(sessionID)

			// For low violations case, we might not get violations, so check differently
			if tc.violationSeverity == "low" && report.TotalViolations == 0 {
				if report.RecommendedAction != "continue" {
					t.Errorf("No violations should recommend continue, got %s", report.RecommendedAction)
				}
				return
			}

			if report.RecommendedAction != tc.expectedAction {
				t.Errorf("Expected recommended action %s, got %s", tc.expectedAction, report.RecommendedAction)
			}
		})
	}
}

// TestIntegrityScoreCalculation validates integrity score calculation logic
func TestIntegrityScoreCalculation(t *testing.T) {
	monitor := NewTimeIntegrityMonitor(nil)

	// Test different violation combinations
	testCases := []struct {
		name             string
		violations       []string // violation severities
		expectedMinScore float64  // minimum expected score
		expectedMaxScore float64  // maximum expected score
	}{
		{"no_violations", []string{}, 100, 100},
		{"single_medium", []string{"medium"}, 90, 100},
		{"single_high", []string{"high"}, 80, 90},
		{"single_critical", []string{"critical"}, 0, 80},
		{"mixed_violations", []string{"medium", "high", "medium"}, 70, 85},
		{"multiple_critical", []string{"critical", "critical"}, 0, 60},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sessionID := "score_test_" + tc.name
			question := &models.Question{
				ID: "score_q", EstimatedTimeSeconds: 60, BloomLevel: "apply", DifficultyLevel: "medium",
			}

			// Create violations based on test case
			for i, severity := range tc.violations {
				var timeSpent int
				switch severity {
				case "critical":
					timeSpent = 350 // 5.8x
				case "high":
					timeSpent = 2 // Below minimum
				case "medium":
					timeSpent = 200 // 3.3x
				}

				// Use slightly different question IDs to avoid duplication
				testQuestion := *question
				testQuestion.ID = question.ID + "_" + string(rune('a'+i))

				monitor.ValidateQuestionTiming(sessionID, &testQuestion, timeSpent, true)
			}

			report := monitor.GetSessionIntegrityReport(sessionID)

			if report.IntegrityScore < tc.expectedMinScore {
				t.Errorf("Integrity score %.2f below expected minimum %.2f",
					report.IntegrityScore, tc.expectedMinScore)
			}

			if report.IntegrityScore > tc.expectedMaxScore {
				t.Errorf("Integrity score %.2f above expected maximum %.2f",
					report.IntegrityScore, tc.expectedMaxScore)
			}
		})
	}
}
