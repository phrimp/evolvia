package service

import (
	"context"

	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type QuizService struct {
	Repo *repository.QuizRepository
}

func NewQuizService(repo *repository.QuizRepository) *QuizService {
	return &QuizService{Repo: repo}
}

func (s *QuizService) ListQuizzes(ctx context.Context) ([]models.Quiz, error) {
	return s.Repo.FindAll(ctx)
}

func (s *QuizService) GetQuiz(ctx context.Context, id string) (*models.Quiz, error) {
	return s.Repo.FindByID(ctx, id)
}

func (s *QuizService) CreateQuiz(ctx context.Context, quiz *models.Quiz) error {
	return s.Repo.Create(ctx, quiz)
}

func (s *QuizService) UpdateQuiz(ctx context.Context, id string, update map[string]interface{}) error {
	return s.Repo.Update(ctx, id, update)
}

func (s *QuizService) DeleteQuiz(ctx context.Context, id string) error {
	return s.Repo.Delete(ctx, id)
}
