package service

import (
	"context"

	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type ResultService struct {
	Repo *repository.ResultRepository
}

func NewResultService(repo *repository.ResultRepository) *ResultService {
	return &ResultService{Repo: repo}
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
	return s.Repo.Create(ctx, result)
}
