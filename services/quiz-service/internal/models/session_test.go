package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// SessionTestSuite defines the test suite for QuizSession model
type SessionTestSuite struct {
	suite.Suite
	sampleSession *QuizSession
}

func (suite *SessionTestSuite) SetupTest() {
	suite.sampleSession = &QuizSession{
		ID:                  "session_123",
		UserID:              "user_456",
		SessionToken:        "token_abc123",
		StartTime:           time.Now(),
		EndTime:             time.Time{},
		DurationSeconds:     300, // 5 minutes
		CurrentStage:        "remember",
		TotalQuestionsAsked: 3,
		StageProgress: map[string]StageProgress{
			"remember": {
				Attempted:     2,
				Correct:       1,
				Passed:        false,
				RecoveryRound: 0,
				Score:         50.0,
			},
		},
		QuestionsUsed:       []string{"q1", "q2"},
		AnsweredQuestionIDs: []string{"q1", "q2"},
		Status:              "active",
		FinalScore:          0.0,
		CompletionType:      "",
		Metadata: map[string]interface{}{
			"skill_id":   "skill_001",
			"skill_name": "Basic Math",
		},
	}
}

func (suite *SessionTestSuite) TestSessionModel_Structure() {
	t := suite.T()
	session := suite.sampleSession

	// Test basic fields
	assert.Equal(t, "session_123", session.ID)
	assert.Equal(t, "user_456", session.UserID)
	assert.Equal(t, "token_abc123", session.SessionToken)
	assert.Equal(t, "active", session.Status)
	assert.Equal(t, "remember", session.CurrentStage)
	assert.Equal(t, 3, session.TotalQuestionsAsked)
	assert.Equal(t, 300, session.DurationSeconds)
	assert.Equal(t, 0.0, session.FinalScore)

	// Test time fields are set
	assert.False(t, session.StartTime.IsZero())
	assert.True(t, session.EndTime.IsZero()) // Not completed yet

	// Test arrays
	assert.Len(t, session.QuestionsUsed, 2)
	assert.Len(t, session.AnsweredQuestionIDs, 2)
	assert.Contains(t, session.QuestionsUsed, "q1")
	assert.Contains(t, session.QuestionsUsed, "q2")
}

func (suite *SessionTestSuite) TestStageProgress_Structure() {
	t := suite.T()

	progress, exists := suite.sampleSession.StageProgress["remember"]
	assert.True(t, exists)
	assert.Equal(t, 2, progress.Attempted)
	assert.Equal(t, 1, progress.Correct)
	assert.False(t, progress.Passed)
	assert.Equal(t, 0, progress.RecoveryRound)
	assert.Equal(t, 50.0, progress.Score)
}

func (suite *SessionTestSuite) TestSessionMetadata_Structure() {
	t := suite.T()

	skillID, exists := suite.sampleSession.Metadata["skill_id"]
	assert.True(t, exists)
	assert.Equal(t, "skill_001", skillID)

	skillName, exists := suite.sampleSession.Metadata["skill_name"]
	assert.True(t, exists)
	assert.Equal(t, "Basic Math", skillName)
}

func (suite *SessionTestSuite) TestSession_JSONSerialization() {
	t := suite.T()

	// Test JSON marshaling
	jsonData, err := json.Marshal(suite.sampleSession)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var deserializedSession QuizSession
	err = json.Unmarshal(jsonData, &deserializedSession)
	assert.NoError(t, err)

	// Compare key fields
	assert.Equal(t, suite.sampleSession.ID, deserializedSession.ID)
	assert.Equal(t, suite.sampleSession.UserID, deserializedSession.UserID)
	assert.Equal(t, suite.sampleSession.Status, deserializedSession.Status)
	assert.Equal(t, suite.sampleSession.CurrentStage, deserializedSession.CurrentStage)
	assert.Equal(t, suite.sampleSession.TotalQuestionsAsked, deserializedSession.TotalQuestionsAsked)
	assert.Equal(t, suite.sampleSession.DurationSeconds, deserializedSession.DurationSeconds)
}

func (suite *SessionTestSuite) TestSession_StatusTransitions() {
	t := suite.T()

	testCases := []struct {
		name          string
		initialStatus string
		newStatus     string
		shouldBeValid bool
		description   string
	}{
		{
			name:          "ActiveToCompleted",
			initialStatus: "active",
			newStatus:     "completed",
			shouldBeValid: true,
			description:   "Should allow transition from active to completed",
		},
		{
			name:          "ActiveToAbandoned",
			initialStatus: "active",
			newStatus:     "abandoned",
			shouldBeValid: true,
			description:   "Should allow transition from active to abandoned",
		},
		{
			name:          "CompletedToActive",
			initialStatus: "completed",
			newStatus:     "active",
			shouldBeValid: false,
			description:   "Should not allow transition from completed to active",
		},
		{
			name:          "StartedToActive",
			initialStatus: "started",
			newStatus:     "active",
			shouldBeValid: true,
			description:   "Should allow transition from started to active",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := &QuizSession{Status: tc.initialStatus}
			isValid := suite.isValidStatusTransition(session.Status, tc.newStatus)
			assert.Equal(t, tc.shouldBeValid, isValid, tc.description)
		})
	}
}

func (suite *SessionTestSuite) TestSession_ProgressCalculations() {
	t := suite.T()

	t.Run("CalculateAccuracy", func(t *testing.T) {
		session := suite.sampleSession
		expectedAccuracy := float64(1) / float64(2) // 1 correct out of 2 attempted
		actualAccuracy := suite.calculateAccuracy(session)
		assert.Equal(t, expectedAccuracy, actualAccuracy)
	})

	t.Run("CalculateOverallProgress", func(t *testing.T) {
		session := suite.sampleSession
		// This would depend on the quiz configuration
		progress := suite.calculateOverallProgress(session)
		assert.GreaterOrEqual(t, progress, 0.0)
		assert.LessOrEqual(t, progress, 100.0)
	})

	t.Run("CalculateAverageTimePerQuestion", func(t *testing.T) {
		session := suite.sampleSession
		if session.TotalQuestionsAsked > 0 {
			expectedAvgTime := float64(session.DurationSeconds) / float64(session.TotalQuestionsAsked)
			actualAvgTime := suite.calculateAverageTimePerQuestion(session)
			assert.Equal(t, expectedAvgTime, actualAvgTime)
		}
	})
}

func (suite *SessionTestSuite) TestStageProgress_Validation() {
	t := suite.T()

	validProgress := StageProgress{
		Attempted:     5,
		Correct:       3,
		Passed:        true,
		RecoveryRound: 0,
		Score:         75.0,
	}

	assert.True(t, suite.validateStageProgress(&validProgress))

	// Test invalid progress
	invalidProgress := StageProgress{
		Attempted:     2,
		Correct:       5, // More correct than attempted
		Passed:        false,
		RecoveryRound: -1,    // Negative recovery round
		Score:         -10.0, // Negative score
	}

	assert.False(t, suite.validateStageProgress(&invalidProgress))
}

func (suite *SessionTestSuite) TestSession_BusinessLogicScenarios() {
	t := suite.T()

	t.Run("SessionTimeout", func(t *testing.T) {
		// Create a session with long duration
		timeoutSession := &QuizSession{
			ID:              "timeout_session",
			Status:          "active",
			StartTime:       time.Now().Add(-2 * time.Hour),
			DurationSeconds: 7200, // 2 hours
		}

		// Session should be considered timed out if it exceeds reasonable duration
		isTimedOut := suite.isSessionTimedOut(timeoutSession, 3600) // 1 hour timeout
		assert.True(t, isTimedOut)
	})

	t.Run("SessionCompletion", func(t *testing.T) {
		completedSession := &QuizSession{
			ID:                  "completed_session",
			Status:              "active",
			TotalQuestionsAsked: 10,
			StageProgress: map[string]StageProgress{
				"remember":   {Attempted: 3, Correct: 2, Passed: true, Score: 80.0},
				"understand": {Attempted: 3, Correct: 3, Passed: true, Score: 100.0},
				"apply":      {Attempted: 4, Correct: 3, Passed: true, Score: 75.0},
			},
		}

		isComplete := suite.isSessionComplete(completedSession)
		assert.True(t, isComplete)
	})

	t.Run("StageCompletion", func(t *testing.T) {
		progress := &StageProgress{
			Attempted:     5,
			Correct:       4,
			Passed:        false,
			RecoveryRound: 0,
			Score:         80.0,
		}

		// Assuming passing threshold is 70%
		shouldBePassed := suite.shouldPassStage(progress, 0.7)
		assert.True(t, shouldBePassed)
	})

	t.Run("RecoveryLogic", func(t *testing.T) {
		failedProgress := &StageProgress{
			Attempted:     3,
			Correct:       1,
			Passed:        false,
			RecoveryRound: 0,
			Score:         33.3,
		}

		needsRecovery := suite.needsRecovery(failedProgress, 0.7)
		assert.True(t, needsRecovery)
	})
}

// Helper methods for validation and calculations
func (suite *SessionTestSuite) isValidStatusTransition(from, to string) bool {
	validTransitions := map[string][]string{
		"started":   {"active", "abandoned"},
		"active":    {"completed", "abandoned", "paused"},
		"paused":    {"active", "abandoned"},
		"completed": {}, // Terminal state
		"abandoned": {}, // Terminal state
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}
	return false
}

func (suite *SessionTestSuite) calculateAccuracy(session *QuizSession) float64 {
	totalCorrect := 0
	totalAttempted := 0

	for _, progress := range session.StageProgress {
		totalCorrect += progress.Correct
		totalAttempted += progress.Attempted
	}

	if totalAttempted == 0 {
		return 0.0
	}
	return float64(totalCorrect) / float64(totalAttempted)
}

func (suite *SessionTestSuite) calculateOverallProgress(session *QuizSession) float64 {
	if len(session.StageProgress) == 0 {
		return 0.0
	}

	completedStages := 0
	totalStages := len(session.StageProgress)

	for _, progress := range session.StageProgress {
		if progress.Passed {
			completedStages++
		}
	}

	return float64(completedStages) / float64(totalStages) * 100
}

func (suite *SessionTestSuite) calculateAverageTimePerQuestion(session *QuizSession) float64 {
	if session.TotalQuestionsAsked == 0 {
		return 0.0
	}
	return float64(session.DurationSeconds) / float64(session.TotalQuestionsAsked)
}

func (suite *SessionTestSuite) validateStageProgress(progress *StageProgress) bool {
	if progress.Attempted < 0 || progress.Correct < 0 {
		return false
	}
	if progress.Correct > progress.Attempted {
		return false
	}
	if progress.RecoveryRound < 0 {
		return false
	}
	if progress.Score < 0 {
		return false
	}
	return true
}

func (suite *SessionTestSuite) isSessionTimedOut(session *QuizSession, timeoutSeconds int) bool {
	return session.DurationSeconds > timeoutSeconds
}

func (suite *SessionTestSuite) isSessionComplete(session *QuizSession) bool {
	// Session is complete if all stages are passed or if explicitly marked as completed
	if session.Status == "completed" {
		return true
	}

	allStagesPassed := true
	for _, progress := range session.StageProgress {
		if !progress.Passed {
			allStagesPassed = false
			break
		}
	}

	return allStagesPassed && len(session.StageProgress) > 0
}

func (suite *SessionTestSuite) shouldPassStage(progress *StageProgress, threshold float64) bool {
	if progress.Attempted == 0 {
		return false
	}
	accuracy := float64(progress.Correct) / float64(progress.Attempted)
	return accuracy >= threshold
}

func (suite *SessionTestSuite) needsRecovery(progress *StageProgress, threshold float64) bool {
	return !progress.Passed && !suite.shouldPassStage(progress, threshold)
}

// TestSessionSuite runs the test suite
func TestSessionSuite(t *testing.T) {
	suite.Run(t, new(SessionTestSuite))
}

// Additional edge case tests
func TestSessionModel_EdgeCases(t *testing.T) {
	t.Run("EmptySession", func(t *testing.T) {
		session := &QuizSession{}
		assert.Empty(t, session.ID)
		assert.Empty(t, session.UserID)
		assert.Equal(t, 0.0, session.FinalScore)
		assert.Equal(t, 0, session.TotalQuestionsAsked)
	})

	t.Run("SessionWithNoProgress", func(t *testing.T) {
		session := &QuizSession{
			ID:            "no_progress",
			StageProgress: make(map[string]StageProgress),
		}

		assert.Empty(t, session.StageProgress)
		assert.Equal(t, 0, session.TotalQuestionsAsked)
	})

	t.Run("SessionWithMaxValues", func(t *testing.T) {
		session := &QuizSession{
			ID:                  "max_values",
			FinalScore:          100.0,
			DurationSeconds:     86400, // 24 hours
			TotalQuestionsAsked: 1000,
		}

		assert.Equal(t, 100.0, session.FinalScore)
		assert.Equal(t, 86400, session.DurationSeconds)
		assert.Equal(t, 1000, session.TotalQuestionsAsked)
	})

	t.Run("SessionMetadataStructure", func(t *testing.T) {
		metadata := SessionMetadata{
			SkillID:       "skill_123",
			SkillName:     "Advanced Programming",
			SkillTags:     []string{"programming", "algorithms", "data-structures"},
			QuizStartTime: time.Now().Unix(),
		}

		assert.Equal(t, "skill_123", metadata.SkillID)
		assert.Equal(t, "Advanced Programming", metadata.SkillName)
		assert.Len(t, metadata.SkillTags, 3)
		assert.NotZero(t, metadata.QuizStartTime)
	})
}
