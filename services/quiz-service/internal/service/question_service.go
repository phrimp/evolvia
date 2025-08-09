package service

import (
	"context"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type QuestionService struct {
	Repo *repository.QuestionRepository
}

func NewQuestionService(repo *repository.QuestionRepository) *QuestionService {
	return &QuestionService{Repo: repo}
}

func (s *QuestionService) ListQuestions(ctx context.Context) ([]models.Question, error) {
	return s.Repo.FindAll(ctx)
}

func (s *QuestionService) GetQuestion(ctx context.Context, id string) (*models.Question, error) {
	return s.Repo.FindByID(ctx, id)
}

func (s *QuestionService) CreateQuestion(ctx context.Context, question *models.Question) error {
	// Ensure Bloom scores are calculated before saving
	question.EnsureBloomScores()
	return s.Repo.Create(ctx, question)
}

func (s *QuestionService) UpdateQuestion(ctx context.Context, id string, update map[string]any) error {
	return s.Repo.Update(ctx, id, update)
}

func (s *QuestionService) DeleteQuestion(ctx context.Context, id string) error {
	return s.Repo.Delete(ctx, id)
}
