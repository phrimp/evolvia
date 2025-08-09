package service

import (
	"context"

	"quiz-service/internal/event"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type ResultService struct {
	Repo      *repository.ResultRepository
	Publisher *event.EventPublisher
}

func NewResultService(repo *repository.ResultRepository, publisher *event.EventPublisher) *ResultService {
	return &ResultService{
		Repo:      repo,
		Publisher: publisher,
	}
}

func (s *ResultService) GetResultBySession(ctx context.Context, sessionID string) (*models.QuizResult, error) {
	return s.Repo.FindBySession(ctx, sessionID)
}

func (s *ResultService) GetResultsByUser(ctx context.Context, userID string) ([]models.QuizResult, error) {
	return s.Repo.FindByUser(ctx, userID)
}

func (s *ResultService) GetResultsByQuiz(ctx context.Context, quizID string) ([]models.QuizResult, error) {
	return s.Repo.FindByQuiz(ctx, quizID)
}

func (s *ResultService) CreateResult(ctx context.Context, result *models.QuizResult) error {
	// Create the result in database
	err := s.Repo.Create(ctx, result)
	if err != nil {
		return err
	}

	// Publish quiz result event for knowledge service consumption
	if s.Publisher != nil {
		eventData := map[string]interface{}{
			"result_id":           result.ID,
			"session_id":          result.SessionID,
			"user_id":             result.UserID,
			"quiz_id":             result.QuizID,
			"final_score":         result.FinalScore,
			"percentage":          result.Percentage,
			"badge_level":         result.BadgeLevel,
			"questions_attempted": result.QuestionsAttempted,
			"questions_correct":   result.QuestionsCorrect,
			"bloom_breakdown":     result.BloomBreakdown,
			"stage_breakdown":     result.StageBreakdown,
			"time_breakdown":      result.TimeBreakdown,
			"completion_type":     result.CompletionType,
			"created_at":          result.CreatedAt,
		}

		err := s.Publisher.Publish("quiz.result.completed", eventData)
		if err != nil {
			// Log the error but don't fail the result creation
			// Event publishing is supplementary to core functionality
			// In production, consider implementing retry logic or dead letter queues
		}
	}

	return nil
}
