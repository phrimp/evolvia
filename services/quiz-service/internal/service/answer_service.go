package service

import (
	"context"

	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type AnswerService struct {
	Repo *repository.AnswerRepository
}

func NewAnswerService(repo *repository.AnswerRepository) *AnswerService {
	return &AnswerService{Repo: repo}
}

func (s *AnswerService) CreateAnswer(ctx context.Context, answer *models.QuizAnswer) error {
	return s.Repo.Create(ctx, answer)
}

func (s *AnswerService) GetAnswersBySession(ctx context.Context, sessionID string) ([]models.QuizAnswer, error) {
	return s.Repo.FindBySession(ctx, sessionID)
}
