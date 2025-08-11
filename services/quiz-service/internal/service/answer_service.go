package service

import (
	"context"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

// DEPRECATED: AnswerService is deprecated in favor of cache-based answer storage in SessionService
// Individual answers are now cached in memory instead of persisted to database
// This service is maintained for backward compatibility and potential rollback scenarios
// Consider removing in future versions after cache-based implementation is proven stable
type AnswerService struct {
	Repo *repository.AnswerRepository
}

// DEPRECATED: Use SessionService.CacheAnswer() instead
func NewAnswerService(repo *repository.AnswerRepository) *AnswerService {
	return &AnswerService{Repo: repo}
}

// DEPRECATED: Individual answers are now cached instead of persisted
// Use SessionService.CacheAnswer() for new implementations
func (s *AnswerService) CreateAnswer(ctx context.Context, answer *models.QuizAnswer) error {
	return s.Repo.Create(ctx, answer)
}

// DEPRECATED: Use SessionService.GetCachedAnswers() instead
// This method still works but accesses potentially stale database data
func (s *AnswerService) GetAnswersBySession(ctx context.Context, sessionID string) ([]models.QuizAnswer, error) {
	return s.Repo.FindBySession(ctx, sessionID)
}
