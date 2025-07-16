package service

import (
	"context"

	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type SessionService struct {
	Repo *repository.SessionRepository
}

func NewSessionService(repo *repository.SessionRepository) *SessionService {
	return &SessionService{Repo: repo}
}

func (s *SessionService) GetSession(ctx context.Context, id string) (*models.QuizSession, error) {
	return s.Repo.FindByID(ctx, id)
}

func (s *SessionService) CreateSession(ctx context.Context, session *models.QuizSession) error {
	return s.Repo.Create(ctx, session)
}

func (s *SessionService) UpdateSession(ctx context.Context, id string, update map[string]interface{}) error {
	return s.Repo.Update(ctx, id, update)
}
