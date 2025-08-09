package service

import (
	"context"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
)

type SkillService struct {
	Repo *repository.SkillRepository
}

func NewSkillService(repo *repository.SkillRepository) *SkillService {
	return &SkillService{
		Repo: repo,
	}
}

// GetAllActiveSkills returns all active skills for user to choose from
func (s *SkillService) GetAllActiveSkills(ctx context.Context) ([]models.Skill, error) {
	return s.Repo.FindActiveSkills(ctx)
}

// GetSkillByID returns a specific skill
func (s *SkillService) GetSkillByID(ctx context.Context, id string) (*models.Skill, error) {
	return s.Repo.FindByID(ctx, id)
}

// GetSkillsByCategory returns skills filtered by category
func (s *SkillService) GetSkillsByCategory(ctx context.Context, categoryID string) ([]models.Skill, error) {
	return s.Repo.FindSkillsByCategory(ctx, categoryID)
}
