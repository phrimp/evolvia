package service

import (
	"context"
	"encoding/json"
	"fmt"
	"quiz-service/internal/adaptive"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"

	"go.mongodb.org/mongo-driver/bson"
)

type SessionService struct {
	Repo            *repository.SessionRepository
	QuizRepo        *repository.QuizRepository
	QuestionRepo    *repository.QuestionRepository
	adaptiveManager *adaptive.Manager
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

// GetNextQuestion gets the next question based on adaptive criteria
func (s *SessionService) GetNextQuestion(ctx context.Context, sessionID string) (*models.Question, error) {
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Reconstruct adaptive session
	adaptiveSession := s.reconstructAdaptiveSession(session)

	// Get criteria for next question
	criteria, err := s.adaptiveManager.GetNextQuestionCriteria(adaptiveSession)
	if err != nil {
		return nil, err
	}

	// Find a question matching criteria
	question, err := s.selectQuestionByCriteria(ctx, session.QuizID, criteria)
	if err != nil {
		return nil, err
	}

	return question, nil
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

// Helper: Select question based on adaptive criteria
func (s *SessionService) selectQuestionByCriteria(ctx context.Context, quizID string, criteria *adaptive.QuestionRequest) (*models.Question, error) {
	// Get all questions for this quiz
	questions, err := s.QuestionRepo.FindByQuizID(ctx, quizID)
	if err != nil {
		return nil, err
	}

	// Map stage to difficulty level
	difficultyLevel := ""
	switch criteria.Stage {
	case adaptive.StageEasy:
		difficultyLevel = "easy"
	case adaptive.StageMedium:
		difficultyLevel = "medium"
	case adaptive.StageHard:
		difficultyLevel = "hard"
	}

	// Filter questions by difficulty and exclude used ones
	var availableQuestions []models.Question
	for _, q := range questions {
		if q.DifficultyLevel != difficultyLevel {
			continue
		}

		// Check if question was already used
		used := false
		for _, usedID := range criteria.ExcludeIDs {
			if q.ID == usedID {
				used = true
				break
			}
		}

		if !used {
			availableQuestions = append(availableQuestions, q)
		}
	}

	if len(availableQuestions) == 0 {
		return nil, fmt.Errorf("no available questions for stage %s", criteria.Stage)
	}

	// For now, return the first available question
	// In production, you'd want random selection or other logic
	return &availableQuestions[0], nil
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
