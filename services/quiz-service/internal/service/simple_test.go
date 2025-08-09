package service

import (
	"quiz-service/internal/adaptive"
	"testing"
)

func TestSimpleBloomBreakdown(t *testing.T) {
	service := &SessionService{}

	// Create a simple session with basic performance data
	session := &adaptive.AdaptiveSession{
		SessionID: "test-session",
		BloomPerformance: map[string]*adaptive.BloomLevelPerformance{
			"apply": {
				QuestionsAttempted: 1,
				QuestionsCorrect:   1,
				ActualScore:        20.0,
				PossibleScore:      20.0,
				AccuracyPercentage: 100.0,
				ScorePercentage:    100.0,
				EfficiencyRating:   "excellent",
			},
		},
	}

	// Test that we can call buildBloomBreakdown
	breakdown := service.buildBloomBreakdown(session)

	// Basic test - just verify the structure is created
	if breakdown.Apply.QuestionsAttempted != 1 {
		t.Errorf("Expected 1 question attempted for apply, got %d", breakdown.Apply.QuestionsAttempted)
	}

	if breakdown.Apply.ScorePercentage != 100.0 {
		t.Errorf("Expected 100%% score percentage for apply, got %f", breakdown.Apply.ScorePercentage)
	}
}

