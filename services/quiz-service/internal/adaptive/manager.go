package adaptive

import (
	"context"
	"fmt"
	"math"
	"quiz-service/internal/models"
	"strings"
	"time"
)

// BloomScoringInterface defines the interface for Bloom scoring services
type BloomScoringInterface interface {
	GetQuestionScore(bloomLevel, difficulty string) int
	GetQuestionScoresByStage(bloomLevel string) map[string]int
}

// ConfigBridgeInterface defines the interface for configuration bridge
type ConfigBridgeInterface interface {
	GetAdaptiveConfig(ctx context.Context) (*AdaptiveConfig, error)
}

// Manager handles adaptive quiz logic
type Manager struct {
	config       *AdaptiveConfig
	bloomScorer  BloomScoringInterface
	configBridge ConfigBridgeInterface // NEW: Database configuration bridge
}

// NewManager creates a new adaptive manager
func NewManager(config *AdaptiveConfig) *Manager {
	if config == nil {
		config = DefaultAdaptiveConfig()
	}
	return &Manager{
		config:      config,
		bloomScorer: nil, // Will be set via SetBloomScorer
	}
}

// NewManagerWithConfigBridge creates adaptive manager with database configuration support
func NewManagerWithConfigBridge(configBridge ConfigBridgeInterface, bloomScorer BloomScoringInterface) *Manager {
	return &Manager{
		config:       nil, // Will be loaded from database
		bloomScorer:  bloomScorer,
		configBridge: configBridge,
	}
}

// NewManagerWithBloomScorer creates a new adaptive manager with Bloom scoring service
func NewManagerWithBloomScorer(config *AdaptiveConfig, bloomScorer BloomScoringInterface) *Manager {
	if config == nil {
		config = DefaultAdaptiveConfig()
	}
	return &Manager{
		config:      config,
		bloomScorer: bloomScorer,
	}
}

// SetBloomScorer sets the Bloom scoring service (for backwards compatibility)
func (m *Manager) SetBloomScorer(bloomScorer BloomScoringInterface) {
	m.bloomScorer = bloomScorer
}

// GetConfig returns the adaptive configuration, loading from database if needed
func (m *Manager) GetConfig() *AdaptiveConfig {
	// Try to load from database first if configBridge available
	if m.configBridge != nil {
		if dbConfig, err := m.configBridge.GetAdaptiveConfig(context.Background()); err == nil {
			m.config = dbConfig // Cache the loaded config
			return dbConfig
		} else {
			// Log warning but continue with cached/default config
			fmt.Printf("[ADAPTIVE_MANAGER] Warning: Failed to load config from database, using cached/default: %v\n", err)
		}
	}

	// Return cached config or default
	if m.config == nil {
		m.config = DefaultAdaptiveConfig()
	}
	return m.config
}

// ProcessAnswer processes an answer and updates the session state with Bloom-aware scoring
func (m *Manager) ProcessAnswer(session *AdaptiveSession, question *models.Question, isCorrect bool) (*AnswerResult, error) {
	if session.IsComplete {
		return nil, fmt.Errorf("session already complete")
	}

	// Get current stage status with nil check
	if session.StageStatuses == nil {
		session.StageStatuses = make(map[Stage]*StageStatus)
	}
	currentStatus := session.StageStatuses[session.CurrentStage]
	if currentStatus == nil {
		currentStatus = &StageStatus{}
		session.StageStatuses[session.CurrentStage] = currentStatus
	}
	// Get current configuration (might load from database)
	currentConfig := m.GetConfig()
	stageConfig := currentConfig.StageConfigs[session.CurrentStage]

	// Update counters
	currentStatus.QuestionsAsked++
	session.TotalQuestionsAsked++

	if isCorrect {
		currentStatus.CorrectAnswers++
	}

	// DEPRECATED: Bloom tracking moved to cached answers approach
	// session.InitializeBloomTracking()

	// Calculate question timing (kept for potential future use)
	_ = 0
	if !session.QuestionStartTime.IsZero() {
		_ = int(time.Since(session.QuestionStartTime).Seconds())
	}

	// Calculate actual score for traditional scoring
	actualScore := m.calculateBloomAwarePoints(question, session.CurrentStage, currentStatus.InRecovery, isCorrect)
	// possibleScore calculation removed - now handled in cached answers approach

	// Update traditional scoring
	currentStatus.Score += actualScore
	session.TotalScore += actualScore

	// DEPRECATED: Bloom performance tracking moved to cached answers approach
	// m.updateBloomPerformance(session, question, actualScore, possibleScore, questionDuration)

	// Reset question timer for next question
	session.QuestionStartTime = time.Now()

	// Determine next action
	result := &AnswerResult{
		IsCorrect:    isCorrect,
		PointsEarned: actualScore,
	}

	// Check if we've hit the max questions limit
	if session.TotalQuestionsAsked >= currentConfig.MaxQuestions {
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

// calculatePoints calculates points based on stage and recovery status (legacy method)
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

// calculateBloomAwarePoints calculates points using centralized Bloom scoring service
func (m *Manager) calculateBloomAwarePoints(question *models.Question, stage Stage, isRecovery bool, isCorrect bool) float64 {
	if !isCorrect {
		return 0
	}

	var baseScore float64

	// Use centralized scoring service (pure service-based approach)
	if m.bloomScorer != nil {
		baseScore = float64(m.bloomScorer.GetQuestionScore(question.BloomLevel, string(stage)))
	} else {
		// ERROR: BloomScorer must be available for consistent scoring
		fmt.Printf("[ADAPTIVE_MANAGER] ERROR: BloomScorer not available for question %s, stage %s\n", question.ID, stage)
		return 0 // Return 0 to indicate scoring failure
	}

	// Apply recovery penalty if in recovery mode
	if isRecovery {
		return baseScore * 0.8 // 20% penalty for recovery mode
	}

	return baseScore
}

// updateBloomPerformance tracks performance metrics per Bloom taxonomy level
func (m *Manager) updateBloomPerformance(
	session *AdaptiveSession,
	question *models.Question,
	actualScore, possibleScore float64,
	duration int,
) {
	bloomLevel := strings.ToLower(question.BloomLevel)

	if session.BloomPerformance[bloomLevel] == nil {
		session.BloomPerformance[bloomLevel] = &BloomLevelPerformance{}
	}

	perf := session.BloomPerformance[bloomLevel]

	// Update counts and scores
	perf.QuestionsAttempted++
	if actualScore > 0 {
		perf.QuestionsCorrect++
	}

	perf.ActualScore += actualScore
	perf.PossibleScore += possibleScore
	perf.TotalTimeSpent += duration

	// Calculate derived metrics
	perf.CalculateMetrics()
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
	// Validate session input to prevent issues
	if session == nil {
		return 0.0
	}

	// Check for invalid session score values
	if math.IsInf(session.TotalScore, 0) || math.IsNaN(session.TotalScore) {
		fmt.Printf("[AdaptiveManager] WARNING: Invalid session total score (%v), defaulting to 0\n", session.TotalScore)
		return 0.0
	}

	// Calculate full session potential score based on MaxQuestions and stage progression
	maxScore := 0.0

	// Get current configuration (might load from database)
	currentConfig := m.GetConfig()

	// Check if MaxQuestions is set, otherwise fallback to stage-based calculation
	if currentConfig.MaxQuestions > 0 {
		// Calculate potential score for the entire session (not just attempted)
		totalQuestionsBudget := float64(currentConfig.MaxQuestions)

		// Calculate total stage questions for weighting
		totalStageQuestions := float64(currentConfig.StageConfigs[StageEasy].InitialQuestions +
			currentConfig.StageConfigs[StageMedium].InitialQuestions +
			currentConfig.StageConfigs[StageHard].InitialQuestions)

		if totalStageQuestions > 0 {
			// Use stage distribution as weights
			easyWeight := float64(currentConfig.StageConfigs[StageEasy].InitialQuestions) / totalStageQuestions
			mediumWeight := float64(currentConfig.StageConfigs[StageMedium].InitialQuestions) / totalStageQuestions
			hardWeight := float64(currentConfig.StageConfigs[StageHard].InitialQuestions) / totalStageQuestions

			weightedAvgPoints := (easyWeight * currentConfig.StageConfigs[StageEasy].BasePoints) +
				(mediumWeight * currentConfig.StageConfigs[StageMedium].BasePoints) +
				(hardWeight * currentConfig.StageConfigs[StageHard].BasePoints)

			// Full session potential = MaxQuestions * weighted average points
			maxScore = totalQuestionsBudget * weightedAvgPoints
		} else {
			// Fallback: use simple average if no stage config
			maxScore = totalQuestionsBudget * 5.0 // default average
		}
	} else {
		// Fallback to traditional stage-based calculation if MaxQuestions not set
		maxScore += float64(currentConfig.StageConfigs[StageEasy].InitialQuestions) * currentConfig.StageConfigs[StageEasy].BasePoints
		maxScore += float64(currentConfig.StageConfigs[StageMedium].InitialQuestions) * currentConfig.StageConfigs[StageMedium].BasePoints
		maxScore += float64(currentConfig.StageConfigs[StageHard].InitialQuestions) * currentConfig.StageConfigs[StageHard].BasePoints
	}

	if maxScore == 0 {
		return 0
	}

	percentage := (session.TotalScore / maxScore) * 100

	// Ensure the result is finite and within reasonable bounds
	if math.IsInf(percentage, 0) || math.IsNaN(percentage) {
		fmt.Printf("[AdaptiveManager] WARNING: Invalid percentage calculated (%v), defaulting to 0\n", percentage)
		return 0.0
	}

	if percentage > 100 {
		return 100
	}
	if percentage < 0 {
		return 0
	}

	return percentage
}

// GetSessionSummary provides a summary of the current session state
func (m *Manager) GetSessionSummary(session *AdaptiveSession) map[string]any {
	return map[string]any{
		"session_id":            session.SessionID,
		"current_stage":         session.CurrentStage,
		"total_questions_asked": session.TotalQuestionsAsked,
		"total_score":           session.TotalScore,
		"final_percentage":      m.CalculateFinalScore(session),
		"is_complete":           session.IsComplete,
		"stages":                session.StageStatuses,
	}
}
