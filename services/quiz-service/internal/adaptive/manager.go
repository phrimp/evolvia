package adaptive

import (
	"fmt"
)

// Manager handles adaptive quiz logic
type Manager struct {
	config *AdaptiveConfig
}

// NewManager creates a new adaptive manager
func NewManager(config *AdaptiveConfig) *Manager {
	if config == nil {
		config = DefaultAdaptiveConfig()
	}
	return &Manager{config: config}
}

// ProcessAnswer processes an answer and updates the session state
func (m *Manager) ProcessAnswer(session *AdaptiveSession, isCorrect bool) (*AnswerResult, error) {
	if session.IsComplete {
		return nil, fmt.Errorf("session already complete")
	}

	// Get current stage status
	currentStatus := session.StageStatuses[session.CurrentStage]
	stageConfig := m.config.StageConfigs[session.CurrentStage]

	// Update counters
	currentStatus.QuestionsAsked++
	session.TotalQuestionsAsked++

	if isCorrect {
		currentStatus.CorrectAnswers++
	}

	// Calculate points
	points := m.calculatePoints(session.CurrentStage, currentStatus.InRecovery, isCorrect)
	currentStatus.Score += points
	session.TotalScore += points

	// Determine next action
	result := &AnswerResult{
		IsCorrect:    isCorrect,
		PointsEarned: points,
	}

	// Check if we've hit the max questions limit
	if session.TotalQuestionsAsked >= m.config.MaxQuestions {
		session.IsComplete = true
		result.IsComplete = true
		return result, nil
	}

	// Determine stage progression
	m.updateStageProgression(session, currentStatus, stageConfig, result)

	return result, nil
}

// updateStageProgression determines if we move to next stage, recovery, or continue
func (m *Manager) updateStageProgression(session *AdaptiveSession, status *StageStatus, config StageConfig, result *AnswerResult) {
	if status.InRecovery {
		// Handle recovery logic
		if status.QuestionsAsked >= config.RecoveryQuestions {
			successRate := float64(status.CorrectAnswers) / float64(status.QuestionsAsked)

			if successRate >= config.RecoveryThreshold {
				// Passed recovery - move to next stage
				status.Passed = true
				m.moveToNextStage(session, result)
			} else {
				// Failed recovery - try another recovery set
				status.RecoveryRound++
				status.QuestionsAsked = 0
				status.CorrectAnswers = 0
				// Stay in recovery mode
			}
		}
	} else {
		// Handle initial questions
		if status.QuestionsAsked >= config.InitialQuestions {
			successRate := float64(status.CorrectAnswers) / float64(status.QuestionsAsked)

			if successRate >= config.PassingThreshold {
				// Passed stage - move to next
				status.Passed = true
				m.moveToNextStage(session, result)
			} else {
				// Failed stage - enter recovery
				status.InRecovery = true
				status.RecoveryRound = 1
				status.QuestionsAsked = 0
				status.CorrectAnswers = 0
			}
		}
	}
}

// moveToNextStage advances to the next difficulty stage
func (m *Manager) moveToNextStage(session *AdaptiveSession, result *AnswerResult) {
	switch session.CurrentStage {
	case StageEasy:
		session.CurrentStage = StageMedium
		result.StageUpdate = true
		result.NextStage = StageMedium
	case StageMedium:
		session.CurrentStage = StageHard
		result.StageUpdate = true
		result.NextStage = StageHard
	case StageHard:
		// Completed all stages
		session.IsComplete = true
		result.IsComplete = true
	}
}

// calculatePoints calculates points based on stage and recovery status
func (m *Manager) calculatePoints(stage Stage, isRecovery bool, isCorrect bool) float64 {
	if !isCorrect {
		return 0
	}

	config := m.config.StageConfigs[stage]
	if isRecovery {
		return config.RecoveryPoints
	}
	return config.BasePoints
}

// GetNextQuestionCriteria determines what type of question is needed next
func (m *Manager) GetNextQuestionCriteria(session *AdaptiveSession) (*QuestionRequest, error) {
	if session.IsComplete {
		return nil, fmt.Errorf("session is complete")
	}

	status := session.StageStatuses[session.CurrentStage]

	return &QuestionRequest{
		SessionID:  session.SessionID,
		Stage:      session.CurrentStage,
		ExcludeIDs: session.UsedQuestionIDs,
		IsRecovery: status.InRecovery,
	}, nil
}

// CalculateFinalScore calculates the final percentage score
func (m *Manager) CalculateFinalScore(session *AdaptiveSession) float64 {
	// Maximum possible score if all stages completed perfectly
	maxScore := 0.0

	// Easy: 5 questions * 3 points = 15
	maxScore += float64(m.config.StageConfigs[StageEasy].InitialQuestions) * m.config.StageConfigs[StageEasy].BasePoints
	// Medium: 5 questions * 7 points = 35
	maxScore += float64(m.config.StageConfigs[StageMedium].InitialQuestions) * m.config.StageConfigs[StageMedium].BasePoints
	// Hard: 5 questions * 10 points = 50
	maxScore += float64(m.config.StageConfigs[StageHard].InitialQuestions) * m.config.StageConfigs[StageHard].BasePoints

	// Total max = 100 points (15 + 35 + 50)

	if maxScore == 0 {
		return 0
	}

	percentage := (session.TotalScore / maxScore) * 100
	if percentage > 100 {
		return 100
	}
	return percentage
}

// GetSessionSummary provides a summary of the current session state
func (m *Manager) GetSessionSummary(session *AdaptiveSession) map[string]interface{} {
	return map[string]interface{}{
		"session_id":            session.SessionID,
		"current_stage":         session.CurrentStage,
		"total_questions_asked": session.TotalQuestionsAsked,
		"total_score":           session.TotalScore,
		"final_percentage":      m.CalculateFinalScore(session),
		"is_complete":           session.IsComplete,
		"stages":                session.StageStatuses,
	}
}
