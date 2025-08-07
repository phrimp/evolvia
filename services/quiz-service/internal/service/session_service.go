// services/quiz-service/internal/service/session_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"quiz-service/internal/adaptive"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
	"quiz-service/internal/selection"

	"go.mongodb.org/mongo-driver/bson"
)

type SessionService struct {
	Repo            *repository.SessionRepository
	QuizRepo        *repository.QuizRepository
	QuestionRepo    *repository.QuestionRepository
	adaptiveManager *adaptive.Manager
	poolManager     *selection.PoolManager
}

func NewSessionService(
	repo *repository.SessionRepository,
	quizRepo *repository.QuizRepository,
	questionRepo *repository.QuestionRepository,
) *SessionService {
	return &SessionService{
		Repo:            repo,
		QuizRepo:        quizRepo,
		QuestionRepo:    questionRepo,
		adaptiveManager: adaptive.NewManager(nil), // Uses default config
		poolManager:     selection.NewPoolManager(questionRepo),
	}
}

func (s *SessionService) GetSession(ctx context.Context, id string) (*models.QuizSession, error) {
	return s.Repo.FindByID(ctx, id)
}

func (s *SessionService) CreateSession(ctx context.Context, session *models.QuizSession) error {
	// Initialize adaptive session state
	adaptiveSession := adaptive.NewAdaptiveSession(session.ID)

	// Store adaptive state in session (as JSON in a field)
	adaptiveData, _ := json.Marshal(adaptiveSession)
	session.Status = "active"
	session.CurrentStage = "easy"

	// Initialize stage progress if not set
	if session.StageProgress == nil {
		session.StageProgress = map[string]models.StageProgress{
			"easy": {
				Attempted: 0,
				Correct:   0,
				Passed:    false,
				Score:     0,
			},
			"medium": {
				Attempted: 0,
				Correct:   0,
				Passed:    false,
				Score:     0,
			},
			"hard": {
				Attempted: 0,
				Correct:   0,
				Passed:    false,
				Score:     0,
			},
		}
	}

	// Store adaptive data in a custom field (you might need to add this to the model)
	fmt.Println(adaptiveData)
	// For now, we'll work with existing fields

	return s.Repo.Create(ctx, session)
}

func (s *SessionService) UpdateSession(ctx context.Context, id string, update map[string]interface{}) error {
	return s.Repo.Update(ctx, id, update)
}

// ProcessAnswer handles answer submission with adaptive logic
func (s *SessionService) ProcessAnswer(ctx context.Context, sessionID string, questionID string, answer string, isCorrect bool) (*adaptive.AnswerResult, error) {
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Reconstruct adaptive session from stored data
	adaptiveSession := s.reconstructAdaptiveSession(session)

	// Process answer through adaptive manager
	result, err := s.adaptiveManager.ProcessAnswer(adaptiveSession, isCorrect)
	if err != nil {
		return nil, err
	}

	// Update session with new state
	s.updateSessionFromAdaptive(session, adaptiveSession, result)

	// Save updated session
	update := bson.M{
		"current_stage":         session.CurrentStage,
		"stage_progress":        session.StageProgress,
		"total_questions_asked": session.TotalQuestionsAsked,
		"questions_used":        append(session.QuestionsUsed, questionID),
		"final_score":           session.FinalScore,
		"status":                session.Status,
	}

	err = s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetNextQuestion gets the next question based on adaptive criteria with weighted selection
func (s *SessionService) GetNextQuestion(ctx context.Context, sessionID string) (*models.Question, error) {
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Get quiz information to extract skill data
	quiz, err := s.QuizRepo.FindByID(ctx, session.QuizID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quiz: %w", err)
	}

	// Get skill information (in production, this would come from skill service)
	skillInfo := s.getSkillInfo(quiz.SkillID)

	// Reconstruct adaptive session
	adaptiveSession := s.reconstructAdaptiveSession(session)

	// Get criteria for next question
	criteria, err := s.adaptiveManager.GetNextQuestionCriteria(adaptiveSession)
	if err != nil {
		return nil, err
	}

	// Find questions using weighted selection
	questions, err := s.selectQuestionsWithWeighting(ctx, session.QuizID, skillInfo, criteria)
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("no available questions for current stage")
	}

	// Return the first selected question
	return &questions[0], nil
}

// Helper: Reconstruct adaptive session from stored session
func (s *SessionService) reconstructAdaptiveSession(session *models.QuizSession) *adaptive.AdaptiveSession {
	adaptiveSession := adaptive.NewAdaptiveSession(session.ID)

	// Map current stage
	switch session.CurrentStage {
	case "easy":
		adaptiveSession.CurrentStage = adaptive.StageEasy
	case "medium":
		adaptiveSession.CurrentStage = adaptive.StageMedium
	case "hard":
		adaptiveSession.CurrentStage = adaptive.StageHard
	}

	// Map stage progress
	for stage, progress := range session.StageProgress {
		var adaptiveStage adaptive.Stage
		switch stage {
		case "easy":
			adaptiveStage = adaptive.StageEasy
		case "medium":
			adaptiveStage = adaptive.StageMedium
		case "hard":
			adaptiveStage = adaptive.StageHard
		}

		adaptiveSession.StageStatuses[adaptiveStage] = &adaptive.StageStatus{
			Stage:          adaptiveStage,
			QuestionsAsked: progress.Attempted,
			CorrectAnswers: progress.Correct,
			InRecovery:     progress.RecoveryRound > 0,
			RecoveryRound:  progress.RecoveryRound,
			Passed:         progress.Passed,
			Score:          progress.Score,
		}
	}

	adaptiveSession.TotalQuestionsAsked = session.TotalQuestionsAsked
	adaptiveSession.UsedQuestionIDs = session.QuestionsUsed
	adaptiveSession.TotalScore = session.FinalScore
	adaptiveSession.IsComplete = session.Status == "completed"

	return adaptiveSession
}

// Helper: Update session from adaptive state
func (s *SessionService) updateSessionFromAdaptive(session *models.QuizSession, adaptiveSession *adaptive.AdaptiveSession, result *adaptive.AnswerResult) {
	// Update current stage
	session.CurrentStage = string(adaptiveSession.CurrentStage)

	// Update stage progress
	for stage, status := range adaptiveSession.StageStatuses {
		session.StageProgress[string(stage)] = models.StageProgress{
			Attempted:     status.QuestionsAsked,
			Correct:       status.CorrectAnswers,
			Passed:        status.Passed,
			RecoveryRound: status.RecoveryRound,
			Score:         status.Score,
		}
	}

	session.TotalQuestionsAsked = adaptiveSession.TotalQuestionsAsked
	session.FinalScore = adaptiveSession.TotalScore

	if adaptiveSession.IsComplete {
		session.Status = "completed"
		session.CompletionType = "adaptive_complete"
	}
}

// Helper: Select questions using weighted selection based on tag matching
func (s *SessionService) selectQuestionsWithWeighting(
	ctx context.Context,
	quizID string,
	skillInfo *selection.SkillInfo,
	criteria *adaptive.QuestionRequest,
) ([]models.Question, error) {
	// Map adaptive stage to difficulty level
	difficulty := ""
	switch criteria.Stage {
	case adaptive.StageEasy:
		difficulty = "easy"
	case adaptive.StageMedium:
		difficulty = "medium"
	case adaptive.StageHard:
		difficulty = "hard"
	}

	// Determine how many questions to select
	count := 1 // Default to 1 for next question

	// Use appropriate selection based on recovery status
	var result *selection.SelectionResult
	var err error

	if criteria.IsRecovery {
		// For recovery, use recovery-optimized selection
		result, err = s.poolManager.SelectRecoveryQuestions(
			ctx,
			quizID,
			skillInfo,
			difficulty,
			count,
			criteria.ExcludeIDs,
		)
	} else {
		// For normal stages, use standard adaptive selection
		result, err = s.poolManager.SelectAdaptiveQuestions(
			ctx,
			quizID,
			skillInfo,
			difficulty,
			count,
			criteria.ExcludeIDs,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to select questions: %w", err)
	}

	return result.Questions, nil
}

// Helper: Get skill information (mock for now, would call skill service in production)
func (s *SessionService) getSkillInfo(skillID string) *selection.SkillInfo {
	// This is mock data based on the Algorithms skill example
	// In production, this would call the skill service

	// Default skill info if not found
	skillInfo := &selection.SkillInfo{
		ID:   skillID,
		Name: "Unknown Skill",
		Tags: []string{},
	}

	// Mock some common skills
	switch skillID {
	case "algorithms", "6878c49ee5903ed1fc67933e":
		skillInfo = &selection.SkillInfo{
			ID:   skillID,
			Name: "Algorithms",
			Tags: []string{
				"computer-science",
				"programming",
				"problem-solving",
				"mathematics",
				"logic",
				"optimization",
			},
			TechnicalTerms: []string{
				"pseudocode",
				"flowchart",
				"complexity analysis",
				"big O notation",
				"sorting algorithm",
				"searching algorithm",
			},
		}
	case "python_programming":
		skillInfo = &selection.SkillInfo{
			ID:   skillID,
			Name: "Python Programming",
			Tags: []string{
				"programming",
				"python",
				"software-development",
				"scripting",
				"object-oriented",
			},
		}
	case "data_structures":
		skillInfo = &selection.SkillInfo{
			ID:   skillID,
			Name: "Data Structures",
			Tags: []string{
				"computer-science",
				"programming",
				"algorithms",
				"memory-management",
				"optimization",
			},
		}
	}

	return skillInfo
}

// Helper: Batch select questions for a stage (useful for pre-loading)
func (s *SessionService) SelectQuestionsForStage(
	ctx context.Context,
	quizID string,
	skillID string,
	stage string,
	count int,
	excludeIDs []string,
) ([]models.Question, error) {
	skillInfo := s.getSkillInfo(skillID)

	result, err := s.poolManager.SelectAdaptiveQuestions(
		ctx,
		quizID,
		skillInfo,
		stage,
		count,
		excludeIDs,
	)
	if err != nil {
		return nil, err
	}

	return result.Questions, nil
}

// GetQuizPoolInfo provides information about question distribution
func (s *SessionService) GetQuizPoolInfo(
	ctx context.Context,
	quizID string,
	skillID string,
) (map[string]interface{}, error) {
	skillInfo := s.getSkillInfo(skillID)

	distribution, err := s.poolManager.GetQuestionDistribution(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Validate if pool is suitable for adaptive quiz
	isValid, counts, _ := s.poolManager.ValidateQuizPool(ctx, quizID, skillInfo)

	distribution["is_valid_for_adaptive"] = isValid
	distribution["question_counts"] = counts

	return distribution, nil
}

// Original methods preserved for compatibility
func (s *SessionService) SubmitSession(ctx context.Context, sessionID, completionType string, finalScore float64) (*models.QuizResult, error) {
	update := map[string]interface{}{
		"status":          "completed",
		"completion_type": completionType,
		"final_score":     finalScore,
	}
	err := s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return nil, err
	}

	result := &models.QuizResult{
		SessionID:      sessionID,
		CompletionType: completionType,
		FinalScore:     finalScore,
	}
	return result, nil
}

func (s *SessionService) PauseSession(ctx context.Context, sessionID, reason string) error {
	update := map[string]interface{}{
		"status":       "paused",
		"pause_reason": reason,
	}
	return s.Repo.Update(ctx, sessionID, update)
}
