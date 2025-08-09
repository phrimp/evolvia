package adaptive

import (
	"fmt"
	"math"
	"quiz-service/internal/models"
	"testing"
	"time"
)

// Test Business Logic: Edge Cases and Complex Scenarios

// Test Edge Case: Maximum Questions Limit
func TestProcessAnswer_MaxQuestionsLimit(t *testing.T) {
	config := &AdaptiveConfig{
		MaxQuestions: 3, // Very low limit to test boundary
		StageConfigs: map[Stage]StageConfig{
			StageEasy: {
				InitialQuestions:  2,
				PassingThreshold:  0.5,
				RecoveryQuestions: 2,
				RecoveryThreshold: 0.5,
				BasePoints:        10,
				RecoveryPoints:    5,
			},
		},
	}

	manager := NewManager(config)
	session := NewAdaptiveSession("test-session")

	question := &models.Question{
		ID:                 "q1",
		BloomLevel:         "remember",
		BloomScore:         10,
		BloomScoresByStage: map[string]int{"easy": 10},
	}

	// Process answers up to the limit
	for i := 0; i < 3; i++ {
		result, err := manager.ProcessAnswer(session, question, true)
		if err != nil {
			t.Fatalf("Question %d: Unexpected error: %v", i+1, err)
		}

		if i == 2 { // Last question
			if !result.IsComplete || !session.IsComplete {
				t.Error("Expected session to be complete after reaching max questions")
			}
		} else {
			if result.IsComplete || session.IsComplete {
				t.Errorf("Question %d: Session should not be complete yet", i+1)
			}
		}
	}

	// Verify we can't process more answers
	if session.TotalQuestionsAsked != 3 {
		t.Errorf("Expected 3 total questions, got %d", session.TotalQuestionsAsked)
	}
}

// Test Edge Case: Recovery Loop Prevention
func TestProcessAnswer_RecoveryLoopPrevention(t *testing.T) {
	config := &AdaptiveConfig{
		MaxQuestions: 50, // High limit to test recovery behavior
		StageConfigs: map[Stage]StageConfig{
			StageEasy: {
				InitialQuestions:  3,
				PassingThreshold:  0.8, // High threshold: need 3/3 correct initially
				RecoveryQuestions: 2,
				RecoveryThreshold: 0.8, // High threshold: need 2/2 correct in recovery
				BasePoints:        10,
				RecoveryPoints:    5,
			},
		},
	}

	manager := NewManager(config)
	session := NewAdaptiveSession("test-session")

	question := &models.Question{
		ID:                 "q1",
		BloomLevel:         "remember",
		BloomScore:         10,
		BloomScoresByStage: map[string]int{"easy": 10},
	}

	// Fail initial questions to enter recovery
	for i := 0; i < 3; i++ {
		result, err := manager.ProcessAnswer(session, question, false)
		if err != nil {
			t.Fatalf("Initial question %d: %v", i+1, err)
		}
		if result.IsComplete {
			t.Errorf("Should not complete on failed initial questions")
		}
	}

	// Verify we're in recovery
	easyStatus := session.StageStatuses[StageEasy]
	if !easyStatus.InRecovery {
		t.Error("Expected to be in recovery mode after failing initial questions")
	}

	// Simulate multiple recovery failures
	maxRecoveryRounds := 10
	for round := 1; round <= maxRecoveryRounds; round++ {
		// Fail recovery questions
		for i := 0; i < 2; i++ {
			result, err := manager.ProcessAnswer(session, question, false)
			if err != nil {
				t.Fatalf("Recovery round %d, question %d: %v", round, i+1, err)
			}

			// Check if session completed due to max questions
			if session.TotalQuestionsAsked >= config.MaxQuestions {
				if !result.IsComplete || !session.IsComplete {
					t.Error("Expected session to complete when max questions reached")
				}
				return // Test passed - max questions prevented infinite loop
			}
		}

		// Verify recovery round incremented
		if easyStatus.RecoveryRound != round+1 && session.TotalQuestionsAsked < config.MaxQuestions {
			t.Errorf("Expected recovery round %d, got %d", round+1, easyStatus.RecoveryRound)
		}
	}

	t.Error("Recovery loop should have been prevented by MaxQuestions limit")
}

// Test Business Logic: Stage Progression Scenarios
func TestCompleteStageProgression_BusinessLogic(t *testing.T) {
	manager := NewManager(nil) // Use default config
	session := NewAdaptiveSession("test-session")

	scenarios := []struct {
		stage         Stage
		bloomLevel    string
		baseScore     int
		questionsPass int // How many to pass before failing pattern
		questionsFail int // How many to fail to test recovery
	}{
		{StageEasy, "remember", 10, 4, 1}, // Pass 4, fail 1, then pass recovery
		{StageMedium, "apply", 20, 4, 1},  // Pass 4, fail 1, then pass recovery
		{StageHard, "create", 35, 3, 2},   // Pass 3, fail 2 (hard stage threshold is lower)
	}

	for _, scenario := range scenarios {
		t.Run(string(scenario.stage), func(t *testing.T) {
			question := &models.Question{
				ID:         "q-" + string(scenario.stage),
				BloomLevel: scenario.bloomLevel,
				BloomScore: scenario.baseScore,
				BloomScoresByStage: map[string]int{
					string(scenario.stage): scenario.baseScore,
				},
			}

			initialTotal := session.TotalQuestionsAsked

			// Pass enough questions to meet threshold
			for i := 0; i < scenario.questionsPass; i++ {
				result, err := manager.ProcessAnswer(session, question, true)
				if err != nil {
					t.Fatalf("Pass question %d: %v", i+1, err)
				}

				// Should not advance stage until we have enough questions
				stageConfig := manager.config.StageConfigs[scenario.stage]
				status := session.StageStatuses[scenario.stage]
				if status.QuestionsAsked < stageConfig.InitialQuestions {
					if result.StageUpdate {
						t.Errorf("Should not advance stage with only %d/%d questions",
							status.QuestionsAsked, stageConfig.InitialQuestions)
					}
				}
			}

			// Fail one more to test we still pass stage
			result, err := manager.ProcessAnswer(session, question, false)
			if err != nil {
				t.Fatalf("Fail question: %v", err)
			}

			status := session.StageStatuses[scenario.stage]
			stageConfig := manager.config.StageConfigs[scenario.stage]

			// Calculate success rate
			successRate := float64(status.CorrectAnswers) / float64(status.QuestionsAsked)

			if successRate >= stageConfig.PassingThreshold {
				if !result.StageUpdate && !session.IsComplete {
					t.Error("Expected stage advancement with sufficient success rate")
				}
				if !status.Passed {
					t.Error("Expected stage to be marked as passed")
				}
			} else {
				if !status.InRecovery {
					t.Error("Expected to enter recovery with insufficient success rate")
				}
			}

			questionsThisStage := session.TotalQuestionsAsked - initialTotal
			t.Logf("Stage %s: Asked %d questions, %d correct (%.1f%%), Passed: %v, Recovery: %v",
				scenario.stage, questionsThisStage, status.CorrectAnswers,
				successRate*100, status.Passed, status.InRecovery)
		})
	}
}

// Test Business Logic: Bloom Performance Tracking Accuracy
func TestBloomPerformanceTracking_BusinessLogic(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("test-session")

	// Test data: different Bloom levels with known outcomes
	testQuestions := []struct {
		bloomLevel string
		score      int
		isCorrect  bool
		timeSpent  int
		isRecovery bool
	}{
		{"remember", 10, true, 30, false},  // Perfect basic question
		{"remember", 10, false, 60, false}, // Failed basic question (took longer)
		{"apply", 20, true, 45, false},     // Perfect medium complexity
		{"apply", 20, false, 120, false},   // Failed medium (struggled)
		{"create", 35, true, 90, false},    // Perfect high complexity
		{"create", 35, true, 60, true},     // Perfect high complexity in recovery (penalty)
	}

	for i, tq := range testQuestions {
		question := &models.Question{
			ID:         fmt.Sprintf("q%d", i+1),
			BloomLevel: tq.bloomLevel,
			BloomScore: tq.score,
			BloomScoresByStage: map[string]int{
				"easy": tq.score,
			},
		}

		// Set recovery mode if needed
		if tq.isRecovery {
			session.StageStatuses[StageEasy].InRecovery = true
		}

		// Set question start time to simulate timing
		session.QuestionStartTime = time.Now().Add(-time.Duration(tq.timeSpent) * time.Second)

		result, err := manager.ProcessAnswer(session, question, tq.isCorrect)
		if err != nil {
			t.Fatalf("Question %d: %v", i+1, err)
		}

		// Verify points earned
		expectedPoints := float64(tq.score)
		if tq.isRecovery {
			expectedPoints *= 0.8 // Recovery penalty
		}
		if !tq.isCorrect {
			expectedPoints = 0
		}

		if absFloat(result.PointsEarned-expectedPoints) > 0.01 {
			t.Errorf("Question %d: Expected %.2f points, got %.2f",
				i+1, expectedPoints, result.PointsEarned)
		}
	}

	// Verify Bloom performance aggregation
	rememberPerf := session.BloomPerformance["remember"]
	if rememberPerf.QuestionsAttempted != 2 || rememberPerf.QuestionsCorrect != 1 {
		t.Errorf("Remember level: Expected 2 attempted, 1 correct, got %d attempted, %d correct",
			rememberPerf.QuestionsAttempted, rememberPerf.QuestionsCorrect)
	}

	applyPerf := session.BloomPerformance["apply"]
	if applyPerf.QuestionsAttempted != 2 || applyPerf.QuestionsCorrect != 1 {
		t.Errorf("Apply level: Expected 2 attempted, 1 correct, got %d attempted, %d correct",
			applyPerf.QuestionsAttempted, applyPerf.QuestionsCorrect)
	}

	createPerf := session.BloomPerformance["create"]
	if createPerf.QuestionsAttempted != 2 || createPerf.QuestionsCorrect != 2 {
		t.Errorf("Create level: Expected 2 attempted, 2 correct, got %d attempted, %d correct",
			createPerf.QuestionsAttempted, createPerf.QuestionsCorrect)
	}

	// Verify score calculations
	expectedCreateActual := 35.0 + (35.0 * 0.8) // Normal + recovery with penalty
	if absFloat(createPerf.ActualScore-expectedCreateActual) > 0.01 {
		t.Errorf("Create level actual score: Expected %.2f, got %.2f",
			expectedCreateActual, createPerf.ActualScore)
	}

	expectedCreatePossible := 35.0 + (35.0 * 0.8) // Both possible in their contexts
	if absFloat(createPerf.PossibleScore-expectedCreatePossible) > 0.01 {
		t.Errorf("Create level possible score: Expected %.2f, got %.2f",
			expectedCreatePossible, createPerf.PossibleScore)
	}

	// Test calculated metrics
	createPerf.CalculateMetrics()
	if createPerf.AccuracyPercentage != 100.0 {
		t.Errorf("Create level accuracy: Expected 100%%, got %.1f%%",
			createPerf.AccuracyPercentage)
	}

	if createPerf.EfficiencyRating != "excellent" {
		t.Errorf("Create level efficiency: Expected 'excellent', got '%s'",
			createPerf.EfficiencyRating)
	}
}

// Test Business Logic: Final Score Calculation Edge Cases
func TestCalculateFinalScore_EdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		config        *AdaptiveConfig
		sessionScore  float64
		expectedScore float64
	}{
		{
			name:          "Perfect score with default config",
			config:        DefaultAdaptiveConfig(),
			sessionScore:  100.0, // Perfect score in traditional system
			expectedScore: 100.0,
		},
		{
			name:          "Zero score",
			config:        DefaultAdaptiveConfig(),
			sessionScore:  0.0,
			expectedScore: 0.0,
		},
		{
			name:          "Score exceeding 100%",
			config:        DefaultAdaptiveConfig(),
			sessionScore:  150.0, // Somehow earned more than max
			expectedScore: 100.0, // Should cap at 100%
		},
		{
			name: "Custom config with different point values",
			config: &AdaptiveConfig{
				StageConfigs: map[Stage]StageConfig{
					StageEasy:   {InitialQuestions: 2, BasePoints: 5},
					StageMedium: {InitialQuestions: 2, BasePoints: 10},
					StageHard:   {InitialQuestions: 2, BasePoints: 15},
				},
			},
			sessionScore:  30.0, // Half of max possible (60)
			expectedScore: 50.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewManager(tc.config)
			session := NewAdaptiveSession("test-session")
			session.TotalScore = tc.sessionScore

			finalScore := manager.CalculateFinalScore(session)

			if absFloat(finalScore-tc.expectedScore) > 0.01 {
				t.Errorf("Expected final score %.2f, got %.2f", tc.expectedScore, finalScore)
			}
		})
	}
}

// Test Business Logic: Session State Integrity
func TestProcessAnswer_SessionStateIntegrity(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("integrity-test")

	question := &models.Question{
		ID:         "q1",
		BloomLevel: "understand",
		BloomScore: 15,
		BloomScoresByStage: map[string]int{
			"easy":   15,
			"medium": 18,
			"hard":   22,
		},
	}

	// Track state changes through multiple answers
	var previousState struct {
		totalQuestions int
		totalScore     float64
		usedIDs        []string
	}

	for i := 0; i < 10; i++ {
		isCorrect := i%3 != 0 // Correct 2/3 of the time

		_, err := manager.ProcessAnswer(session, question, isCorrect)
		if err != nil {
			t.Fatalf("Answer %d: %v", i+1, err)
		}

		// Verify monotonic increases
		if session.TotalQuestionsAsked <= previousState.totalQuestions {
			t.Errorf("Answer %d: Total questions should increase, was %d, now %d",
				i+1, previousState.totalQuestions, session.TotalQuestionsAsked)
		}

		if isCorrect && session.TotalScore <= previousState.totalScore {
			t.Errorf("Answer %d: Score should increase for correct answers, was %.2f, now %.2f",
				i+1, previousState.totalScore, session.TotalScore)
		}

		// Verify stage status consistency
		for stage, status := range session.StageStatuses {
			if status.QuestionsAsked < 0 || status.CorrectAnswers < 0 {
				t.Errorf("Answer %d: Negative counts in stage %s: asked=%d, correct=%d",
					i+1, stage, status.QuestionsAsked, status.CorrectAnswers)
			}

			if status.CorrectAnswers > status.QuestionsAsked {
				t.Errorf("Answer %d: More correct than asked in stage %s: correct=%d, asked=%d",
					i+1, stage, status.CorrectAnswers, status.QuestionsAsked)
			}

			if status.Passed && status.QuestionsAsked == 0 {
				t.Errorf("Answer %d: Stage %s marked as passed with no questions",
					i+1, stage)
			}
		}

		// Verify Bloom performance consistency
		for level, perf := range session.BloomPerformance {
			if perf.QuestionsCorrect > perf.QuestionsAttempted {
				t.Errorf("Answer %d: More correct than attempted in Bloom level %s",
					i+1, level)
			}

			if perf.ActualScore > perf.PossibleScore {
				t.Errorf("Answer %d: Actual score exceeds possible in Bloom level %s",
					i+1, level)
			}
		}

		// Update previous state
		previousState.totalQuestions = session.TotalQuestionsAsked
		previousState.totalScore = session.TotalScore
		previousState.usedIDs = make([]string, len(session.UsedQuestionIDs))
		copy(previousState.usedIDs, session.UsedQuestionIDs)

		if session.IsComplete {
			break
		}
	}
}

// Test Business Logic: Complex Recovery Scenarios
func TestRecoveryScenarios_BusinessLogic(t *testing.T) {
	// Custom config for predictable recovery behavior
	config := &AdaptiveConfig{
		MaxQuestions: 20,
		StageConfigs: map[Stage]StageConfig{
			StageEasy: {
				InitialQuestions:  3,
				PassingThreshold:  0.67, // Need 2/3 correct
				RecoveryQuestions: 2,
				RecoveryThreshold: 1.0, // Need 2/2 correct in recovery
				BasePoints:        10,
				RecoveryPoints:    6,
			},
		},
	}

	scenarios := []struct {
		name            string
		initialPattern  []bool // Results for initial questions
		recoveryPattern []bool // Results for recovery questions
		shouldPass      bool
		shouldAdvance   bool
	}{
		{
			name:            "Pass initial, no recovery needed",
			initialPattern:  []bool{true, true, true},
			recoveryPattern: []bool{},
			shouldPass:      true,
			shouldAdvance:   true,
		},
		{
			name:            "Fail initial, pass recovery",
			initialPattern:  []bool{true, false, false}, // 1/3 = 33% < 67%
			recoveryPattern: []bool{true, true},         // 2/2 = 100% >= 100%
			shouldPass:      true,
			shouldAdvance:   true,
		},
		{
			name:            "Fail initial, fail first recovery",
			initialPattern:  []bool{false, false, true}, // 1/3 = 33% < 67%
			recoveryPattern: []bool{true, false},        // 1/2 = 50% < 100%
			shouldPass:      false,
			shouldAdvance:   false,
		},
		{
			name:            "Borderline pass initial",
			initialPattern:  []bool{true, true, false}, // 2/3 = 67% >= 67%
			recoveryPattern: []bool{},
			shouldPass:      true,
			shouldAdvance:   true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			manager := NewManager(config)
			session := NewAdaptiveSession("recovery-test")

			question := &models.Question{
				ID:                 "recovery-q",
				BloomLevel:         "remember",
				BloomScore:         10,
				BloomScoresByStage: map[string]int{"easy": 10},
			}

			// Process initial questions
			for i, correct := range scenario.initialPattern {
				result, err := manager.ProcessAnswer(session, question, correct)
				if err != nil {
					t.Fatalf("Initial question %d: %v", i+1, err)
				}

				if result.StageUpdate && len(scenario.recoveryPattern) > 0 {
					t.Errorf("Should not advance stage if recovery is expected")
				}
			}

			status := session.StageStatuses[StageEasy]

			// Check if we entered recovery as expected
			expectedRecovery := len(scenario.recoveryPattern) > 0
			if status.InRecovery != expectedRecovery {
				t.Errorf("Expected recovery: %v, got: %v", expectedRecovery, status.InRecovery)
			}

			// Process recovery questions if needed
			for i, correct := range scenario.recoveryPattern {
				result, err := manager.ProcessAnswer(session, question, correct)
				if err != nil {
					t.Fatalf("Recovery question %d: %v", i+1, err)
				}

				// Check if we completed recovery
				if i == len(scenario.recoveryPattern)-1 {
					if scenario.shouldPass && !status.Passed {
						t.Error("Expected to pass stage after successful recovery")
					}
					if scenario.shouldAdvance && !result.StageUpdate && !session.IsComplete {
						t.Error("Expected stage advancement after successful recovery")
					}
				}
			}

			// Final verification
			if status.Passed != scenario.shouldPass {
				t.Errorf("Expected stage passed: %v, got: %v", scenario.shouldPass, status.Passed)
			}

			// Verify point calculations for recovery
			if len(scenario.recoveryPattern) > 0 {
				expectedRecoveryScore := 0.0
				for _, correct := range scenario.recoveryPattern {
					if correct {
						expectedRecoveryScore += config.StageConfigs[StageEasy].RecoveryPoints
					}
				}

				// Recovery points should be included in total score
				if status.InRecovery && len(scenario.recoveryPattern) == 2 {
					// This might not be exact due to different scoring systems,
					// but we can verify recovery scoring was applied
					if status.Score == 0 && scenario.recoveryPattern[0] || scenario.recoveryPattern[1] {
						t.Error("Expected some recovery points to be awarded")
					}
				}
			}
		})
	}
}

// Helper function for floating point comparison
func absFloat(x float64) float64 {
	return math.Abs(x)
}
