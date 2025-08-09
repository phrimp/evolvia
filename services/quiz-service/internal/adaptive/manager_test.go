package adaptive

import (
	"math"
	"quiz-service/internal/models"
	"testing"
	"time"
)

// Helper function for absolute value
func abs(x float64) float64 {
	return math.Abs(x)
}

func TestBloomAwarePointsCalculation(t *testing.T) {
	manager := NewManager(nil) // Use default config

	testCases := []struct {
		name           string
		bloomLevel     string
		stage          Stage
		isRecovery     bool
		isCorrect      bool
		expectedPoints float64
	}{
		{"remember easy correct", "remember", StageEasy, false, true, 10.0},
		{"remember medium correct", "remember", StageMedium, false, true, 12.0}, // 10 * 1.2
		{"remember hard correct", "remember", StageHard, false, true, 15.0},     // 10 * 1.5
		{"create easy correct", "create", StageEasy, false, true, 35.0},
		{"create medium correct", "create", StageMedium, false, true, 42.0},       // 35 * 1.2
		{"create hard correct", "create", StageHard, false, true, 52.0},           // 35 * 1.5
		{"apply medium recovery correct", "apply", StageMedium, true, true, 19.2}, // (20 * 1.2) * 0.8
		{"analyze hard incorrect", "analyze", StageHard, false, false, 0.0},
		{"evaluate easy recovery incorrect", "evaluate", StageEasy, true, false, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			question := &models.Question{
				BloomLevel: tc.bloomLevel,
			}
			question.EnsureBloomScores()

			points := manager.calculateBloomAwarePoints(question, tc.stage, tc.isRecovery, tc.isCorrect)

			// Use small epsilon for floating point comparison
			epsilon := 0.01
			if abs(points-tc.expectedPoints) > epsilon {
				t.Errorf("Expected points %.2f, got %.2f", tc.expectedPoints, points)
			}
		})
	}
}

func TestProcessAnswerWithBloomScoring(t *testing.T) {
	manager := NewManager(nil) // Use default config

	session := NewAdaptiveSession("test-session")
	question := &models.Question{
		BloomLevel: "analyze",
	}
	question.EnsureBloomScores()

	// Test correct answer
	result, err := manager.ProcessAnswer(session, question, true)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedPoints := 25.0 // analyze level on easy stage
	if result.PointsEarned != expectedPoints {
		t.Errorf("Expected points earned %.1f, got %.1f", expectedPoints, result.PointsEarned)
	}

	if !result.IsCorrect {
		t.Error("Expected IsCorrect to be true")
	}

	// Test incorrect answer
	result2, err := manager.ProcessAnswer(session, question, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result2.PointsEarned != 0.0 {
		t.Errorf("Expected 0 points for incorrect answer, got %.1f", result2.PointsEarned)
	}

	if result2.IsCorrect {
		t.Error("Expected IsCorrect to be false")
	}
}

func TestCompareOldVsNewScoring(t *testing.T) {
	manager := NewManager(nil)

	question := &models.Question{
		BloomLevel: "create", // Highest cognitive level
	}
	question.EnsureBloomScores()

	// Old scoring: fixed points based on stage only
	oldPoints := manager.calculatePoints(StageEasy, false, true)

	// New Bloom-aware scoring: uses question's Bloom complexity
	newPoints := manager.calculateBloomAwarePoints(question, StageEasy, false, true)

	t.Logf("Old scoring (stage-based): %.1f points", oldPoints)
	t.Logf("New scoring (Bloom-aware): %.1f points", newPoints)

	// New scoring should reward higher cognitive complexity
	if newPoints <= oldPoints {
		t.Errorf("Expected new Bloom-aware scoring (%.1f) to be higher than old stage-based scoring (%.1f) for 'create' level questions", newPoints, oldPoints)
	}

	// Test with lowest cognitive level
	lowQuestion := &models.Question{
		BloomLevel: "remember",
	}
	lowQuestion.EnsureBloomScores()

	lowNewPoints := manager.calculateBloomAwarePoints(lowQuestion, StageEasy, false, true)

	if lowNewPoints >= newPoints {
		t.Errorf("Expected 'remember' level (%.1f) to have lower points than 'create' level (%.1f)", lowNewPoints, newPoints)
	}
}

func TestProcessAnswer_BloomTracking(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("test-session")

	// Create a test question
	question := &models.Question{
		ID:         "q1",
		BloomLevel: "apply",
		BloomScore: 20,
		BloomScoresByStage: map[string]int{
			"easy":   20,
			"medium": 24,
			"hard":   30,
		},
	}

	// Set question start time
	session.QuestionStartTime = time.Now().Add(-30 * time.Second)

	// Process correct answer
	result, err := manager.ProcessAnswer(session, question, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify basic result
	if !result.IsCorrect {
		t.Error("Expected correct answer")
	}

	if result.PointsEarned != 20.0 {
		t.Errorf("Expected 20 points, got %f", result.PointsEarned)
	}

	// Verify Bloom tracking
	applyPerf := session.BloomPerformance["apply"]
	if applyPerf == nil {
		t.Fatal("Expected apply performance tracking")
	}

	if applyPerf.QuestionsAttempted != 1 {
		t.Errorf("Expected 1 question attempted, got %d", applyPerf.QuestionsAttempted)
	}

	if applyPerf.QuestionsCorrect != 1 {
		t.Errorf("Expected 1 question correct, got %d", applyPerf.QuestionsCorrect)
	}

	if applyPerf.ActualScore != 20.0 {
		t.Errorf("Expected actual score 20, got %f", applyPerf.ActualScore)
	}

	if applyPerf.PossibleScore != 20.0 {
		t.Errorf("Expected possible score 20, got %f", applyPerf.PossibleScore)
	}

	if applyPerf.AccuracyPercentage != 100.0 {
		t.Errorf("Expected 100%% accuracy, got %f", applyPerf.AccuracyPercentage)
	}

	if applyPerf.ScorePercentage != 100.0 {
		t.Errorf("Expected 100%% score percentage, got %f", applyPerf.ScorePercentage)
	}

	if applyPerf.EfficiencyRating != "excellent" {
		t.Errorf("Expected 'excellent' rating, got %s", applyPerf.EfficiencyRating)
	}
}

func TestProcessAnswer_IncorrectAnswer_BloomTracking(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("test-session")

	question := &models.Question{
		ID:         "q2",
		BloomLevel: "analyze",
		BloomScore: 25,
		BloomScoresByStage: map[string]int{
			"easy":   25,
			"medium": 30,
			"hard":   37,
		},
	}

	session.QuestionStartTime = time.Now().Add(-45 * time.Second)

	// Process incorrect answer
	result, err := manager.ProcessAnswer(session, question, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify basic result
	if result.IsCorrect {
		t.Error("Expected incorrect answer")
	}

	if result.PointsEarned != 0.0 {
		t.Errorf("Expected 0 points, got %f", result.PointsEarned)
	}

	// Verify Bloom tracking
	analyzePerf := session.BloomPerformance["analyze"]
	if analyzePerf == nil {
		t.Fatal("Expected analyze performance tracking")
	}

	if analyzePerf.QuestionsAttempted != 1 {
		t.Errorf("Expected 1 question attempted, got %d", analyzePerf.QuestionsAttempted)
	}

	if analyzePerf.QuestionsCorrect != 0 {
		t.Errorf("Expected 0 questions correct, got %d", analyzePerf.QuestionsCorrect)
	}

	if analyzePerf.ActualScore != 0.0 {
		t.Errorf("Expected actual score 0, got %f", analyzePerf.ActualScore)
	}

	if analyzePerf.PossibleScore != 25.0 {
		t.Errorf("Expected possible score 25, got %f", analyzePerf.PossibleScore)
	}

	if analyzePerf.AccuracyPercentage != 0.0 {
		t.Errorf("Expected 0%% accuracy, got %f", analyzePerf.AccuracyPercentage)
	}

	if analyzePerf.ScorePercentage != 0.0 {
		t.Errorf("Expected 0%% score percentage, got %f", analyzePerf.ScorePercentage)
	}

	if analyzePerf.EfficiencyRating != "needs_improvement" {
		t.Errorf("Expected 'needs_improvement' rating, got %s", analyzePerf.EfficiencyRating)
	}
}

func TestProcessAnswer_RecoveryMode_BloomTracking(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("test-session")

	// Set session to recovery mode
	session.StageStatuses[StageEasy].InRecovery = true

	question := &models.Question{
		ID:         "q3",
		BloomLevel: "remember",
		BloomScore: 10,
		BloomScoresByStage: map[string]int{
			"easy":   10,
			"medium": 12,
			"hard":   15,
		},
	}

	session.QuestionStartTime = time.Now().Add(-20 * time.Second)

	// Process correct answer in recovery
	result, err := manager.ProcessAnswer(session, question, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify recovery penalty applied (20% penalty)
	expectedScore := 10.0 * 0.8 // 8.0
	if result.PointsEarned != expectedScore {
		t.Errorf("Expected %f points (with recovery penalty), got %f", expectedScore, result.PointsEarned)
	}

	// Verify Bloom tracking
	rememberPerf := session.BloomPerformance["remember"]
	if rememberPerf.ActualScore != expectedScore {
		t.Errorf("Expected actual score %f, got %f", expectedScore, rememberPerf.ActualScore)
	}

	// In recovery mode, possible score should also reflect the penalty
	// because that's the maximum that could be earned in this context
	if rememberPerf.PossibleScore != expectedScore {
		t.Errorf("Expected possible score %f (with recovery penalty), got %f", expectedScore, rememberPerf.PossibleScore)
	}

	// Score percentage should be 100% since we got the max possible in recovery mode
	if rememberPerf.ScorePercentage != 100.0 {
		t.Errorf("Expected 100%% score percentage (actual/possible in recovery), got %f", rememberPerf.ScorePercentage)
	}
}

func TestProcessAnswer_MultipleBloomLevels(t *testing.T) {
	manager := NewManager(nil)
	session := NewAdaptiveSession("test-session")

	// Test questions for different Bloom levels
	questions := []*models.Question{
		{
			ID:                 "q1",
			BloomLevel:         "remember",
			BloomScore:         10,
			BloomScoresByStage: map[string]int{"easy": 10},
		},
		{
			ID:                 "q2",
			BloomLevel:         "understand",
			BloomScore:         15,
			BloomScoresByStage: map[string]int{"easy": 15},
		},
		{
			ID:                 "q3",
			BloomLevel:         "apply",
			BloomScore:         20,
			BloomScoresByStage: map[string]int{"easy": 20},
		},
	}

	answers := []bool{true, false, true}

	// Process all questions
	for i, question := range questions {
		session.QuestionStartTime = time.Now().Add(-30 * time.Second)

		_, err := manager.ProcessAnswer(session, question, answers[i])
		if err != nil {
			t.Fatalf("Expected no error for question %d, got %v", i, err)
		}
	}

	// Verify individual Bloom level tracking
	rememberPerf := session.BloomPerformance["remember"]
	if rememberPerf.QuestionsAttempted != 1 || rememberPerf.QuestionsCorrect != 1 {
		t.Error("Remember level tracking incorrect")
	}

	understandPerf := session.BloomPerformance["understand"]
	if understandPerf.QuestionsAttempted != 1 || understandPerf.QuestionsCorrect != 0 {
		t.Error("Understand level tracking incorrect")
	}

	applyPerf := session.BloomPerformance["apply"]
	if applyPerf.QuestionsAttempted != 1 || applyPerf.QuestionsCorrect != 1 {
		t.Error("Apply level tracking incorrect")
	}

	// Verify total scores
	expectedTotalActual := 10.0 + 0.0 + 20.0 // 30.0
	actualTotal := rememberPerf.ActualScore + understandPerf.ActualScore + applyPerf.ActualScore
	if actualTotal != expectedTotalActual {
		t.Errorf("Expected total actual score %f, got %f", expectedTotalActual, actualTotal)
	}

	expectedTotalPossible := 10.0 + 15.0 + 20.0 // 45.0
	possibleTotal := rememberPerf.PossibleScore + understandPerf.PossibleScore + applyPerf.PossibleScore
	if possibleTotal != expectedTotalPossible {
		t.Errorf("Expected total possible score %f, got %f", expectedTotalPossible, possibleTotal)
	}
}

func TestBloomLevelPerformance_CalculateMetrics(t *testing.T) {
	perf := &BloomLevelPerformance{
		QuestionsAttempted: 4,
		QuestionsCorrect:   3,
		ActualScore:        75.0,
		PossibleScore:      100.0,
		TotalTimeSpent:     120, // 2 minutes
	}

	perf.CalculateMetrics()

	// Test accuracy percentage
	expectedAccuracy := 75.0 // 3/4 * 100
	if perf.AccuracyPercentage != expectedAccuracy {
		t.Errorf("Expected accuracy %f, got %f", expectedAccuracy, perf.AccuracyPercentage)
	}

	// Test score percentage
	expectedScorePercentage := 75.0 // 75/100 * 100
	if perf.ScorePercentage != expectedScorePercentage {
		t.Errorf("Expected score percentage %f, got %f", expectedScorePercentage, perf.ScorePercentage)
	}

	// Test average question score
	expectedAvgScore := 18.75 // 75/4
	if perf.AverageQuestionScore != expectedAvgScore {
		t.Errorf("Expected avg question score %f, got %f", expectedAvgScore, perf.AverageQuestionScore)
	}

	// Test average time per question
	expectedAvgTime := 30.0 // 120/4
	if perf.AverageTimePerQ != expectedAvgTime {
		t.Errorf("Expected avg time per question %f, got %f", expectedAvgTime, perf.AverageTimePerQ)
	}

	// Test efficiency rating
	expectedRating := "good" // 75% falls in 70-85 range
	if perf.EfficiencyRating != expectedRating {
		t.Errorf("Expected efficiency rating %s, got %s", expectedRating, perf.EfficiencyRating)
	}
}

func TestBloomLevelPerformance_EfficiencyRatings(t *testing.T) {
	testCases := []struct {
		scorePercentage float64
		expectedRating  string
	}{
		{95.0, "excellent"},
		{85.0, "excellent"},
		{84.9, "good"},
		{75.0, "good"},
		{70.0, "good"},
		{69.9, "satisfactory"},
		{55.0, "satisfactory"},
		{50.0, "satisfactory"},
		{49.9, "needs_improvement"},
		{25.0, "needs_improvement"},
		{0.0, "needs_improvement"},
	}

	for _, tc := range testCases {
		perf := &BloomLevelPerformance{
			QuestionsAttempted: 1,
			ActualScore:        tc.scorePercentage,
			PossibleScore:      100.0,
		}

		perf.CalculateMetrics()

		if perf.EfficiencyRating != tc.expectedRating {
			t.Errorf("Score %f: expected rating %s, got %s",
				tc.scorePercentage, tc.expectedRating, perf.EfficiencyRating)
		}
	}
}

func TestInitializeBloomTracking(t *testing.T) {
	session := NewAdaptiveSession("test")

	// Verify all Bloom levels are initialized
	expectedLevels := []string{"remember", "understand", "apply", "analyze", "evaluate", "create"}

	for _, level := range expectedLevels {
		if session.BloomPerformance[level] == nil {
			t.Errorf("Expected %s level to be initialized", level)
		}

		perf := session.BloomPerformance[level]
		if perf.QuestionsAttempted != 0 || perf.ActualScore != 0 {
			t.Errorf("Expected %s level to start with zero values", level)
		}
	}
}
