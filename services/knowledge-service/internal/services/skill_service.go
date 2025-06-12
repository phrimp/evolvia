package service

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SkillService struct {
	repo *repository.SkillRepository
}

// NewSkillService creates a new skill service and initializes data
func NewSkillService(skillRepo *repository.SkillRepository, dataDir string) (*SkillService, error) {
	service := &SkillService{
		repo: skillRepo,
	}

	// Initialize the service
	if err := service.initialize(context.Background(), dataDir); err != nil {
		return nil, fmt.Errorf("failed to initialize skill service: %w", err)
	}

	return service, nil
}

// initialize sets up indexes and loads initial data
func (s *SkillService) initialize(ctx context.Context, dataDir string) error {
	log.Println("Initializing Skill Service...")

	// Create database indexes
	log.Println("Creating database indexes...")
	if err := s.repo.InitializeIndexes(ctx); err != nil {
		return fmt.Errorf("failed to initialize indexes: %w", err)
	}
	log.Println("Database indexes created successfully")

	// Load initial data from /data/skills/*
	log.Println("Loading skill data from directory:", dataDir)
	if err := s.repo.InitializeData(ctx, dataDir); err != nil {
		return fmt.Errorf("failed to initialize data: %w", err)
	}
	log.Println("Skill data loaded successfully")

	return nil
}

// CRUD Operations

// CreateSkill creates a new skill
func (s *SkillService) CreateSkill(ctx context.Context, skill *models.Skill) (*models.Skill, error) {
	// Validate skill
	if err := s.validateSkill(skill); err != nil {
		return nil, fmt.Errorf("skill validation failed: %w", err)
	}

	// Check if skill already exists
	exists, err := s.repo.ExistsByName(ctx, skill.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check skill existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("skill with name '%s' already exists", skill.Name)
	}

	return s.repo.Create(ctx, skill)
}

// GetSkillByID retrieves a skill by ID
func (s *SkillService) GetSkillByID(ctx context.Context, id bson.ObjectID) (*models.Skill, error) {
	skill, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Increment usage count
	go func() {
		if err := s.repo.IncrementUsageCount(context.Background(), id); err != nil {
			log.Printf("Failed to increment usage count for skill %s: %v", id.Hex(), err)
		}
	}()

	return skill, nil
}

// GetSkillByName retrieves a skill by name
func (s *SkillService) GetSkillByName(ctx context.Context, name string) (*models.Skill, error) {
	skill, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Increment usage count
	go func() {
		if err := s.repo.IncrementUsageCount(context.Background(), skill.ID); err != nil {
			log.Printf("Failed to increment usage count for skill %s: %v", skill.ID.Hex(), err)
		}
	}()

	return skill, nil
}

// UpdateSkill updates an existing skill
func (s *SkillService) UpdateSkill(ctx context.Context, id bson.ObjectID, skill *models.Skill) (*models.Skill, error) {
	// Check if skill exists
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Validate updated skill
	if err := s.validateSkill(skill); err != nil {
		return nil, fmt.Errorf("skill validation failed: %w", err)
	}

	// Check if name is being changed and if new name already exists
	if skill.Name != existing.Name {
		exists, err := s.repo.ExistsByName(ctx, skill.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check skill existence: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("skill with name '%s' already exists", skill.Name)
		}
	}

	return s.repo.Update(ctx, id, skill)
}

// DeleteSkill soft deletes a skill
func (s *SkillService) DeleteSkill(ctx context.Context, id bson.ObjectID) error {
	// Check if skill exists
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("skill not found")
	}

	return s.repo.Delete(ctx, id)
}

// ListSkills retrieves skills with filtering and pagination
func (s *SkillService) ListSkills(ctx context.Context, opts repository.ListOptions) ([]*models.Skill, int64, error) {
	// Set default active only filter
	if !opts.ActiveOnly {
		opts.ActiveOnly = true
	}

	return s.repo.List(ctx, opts)
}

// SearchSkills searches for skills by keywords
func (s *SkillService) SearchSkills(ctx context.Context, keywords string, limit int) ([]*models.Skill, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	return s.repo.SearchByKeywords(ctx, keywords, limit)
}

// GetSkillsByCategory retrieves skills by category
func (s *SkillService) GetSkillsByCategory(ctx context.Context, categoryID bson.ObjectID) ([]*models.Skill, error) {
	return s.repo.GetByCategory(ctx, categoryID)
}

// GetMostUsedSkills retrieves the most frequently used skills
func (s *SkillService) GetMostUsedSkills(ctx context.Context, limit int) ([]*models.Skill, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	return s.repo.GetMostUsed(ctx, limit)
}

// GetRelatedSkills finds skills related to the given skill
func (s *SkillService) GetRelatedSkills(ctx context.Context, skillID bson.ObjectID, relationType models.RelationType) ([]*models.Skill, error) {
	return s.repo.GetRelatedSkills(ctx, skillID, relationType)
}

// BatchCreateSkills creates multiple skills at once
func (s *SkillService) BatchCreateSkills(ctx context.Context, skills []*models.Skill) error {
	// Validate all skills
	for i, skill := range skills {
		if err := s.validateSkill(skill); err != nil {
			return fmt.Errorf("skill validation failed at index %d: %w", i, err)
		}
	}

	// Check for duplicate names
	nameMap := make(map[string]bool)
	for i, skill := range skills {
		if nameMap[skill.Name] {
			return fmt.Errorf("duplicate skill name '%s' at index %d", skill.Name, i)
		}
		nameMap[skill.Name] = true

		// Check against existing skills
		exists, err := s.repo.ExistsByName(ctx, skill.Name)
		if err != nil {
			return fmt.Errorf("failed to check skill existence: %w", err)
		}
		if exists {
			return fmt.Errorf("skill with name '%s' already exists", skill.Name)
		}
	}

	return s.repo.BatchCreate(ctx, skills)
}

// ReloadDataFromFiles reloads skill data from the data directory
func (s *SkillService) ReloadDataFromFiles(ctx context.Context, dataDir string) error {
	log.Println("Reloading skill data from directory:", dataDir)
	return s.repo.InitializeData(ctx, dataDir)
}

// validateSkill performs basic validation on skill data
func (s *SkillService) validateSkill(skill *models.Skill) error {
	if skill == nil {
		return fmt.Errorf("skill cannot be nil")
	}

	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	if skill.Description == "" {
		return fmt.Errorf("skill description is required")
	}

	// Validate identification rules
	if len(skill.IdentificationRules.PrimaryPatterns) == 0 {
		return fmt.Errorf("at least one primary pattern is required")
	}

	// Validate pattern weights
	for _, pattern := range skill.IdentificationRules.PrimaryPatterns {
		if pattern.Weight < 0 || pattern.Weight > 1 {
			return fmt.Errorf("pattern weight must be between 0 and 1")
		}
	}

	for _, pattern := range skill.IdentificationRules.SecondaryPatterns {
		if pattern.Weight < 0 || pattern.Weight > 1 {
			return fmt.Errorf("pattern weight must be between 0 and 1")
		}
	}

	// Validate metadata
	if skill.Metadata.Difficulty < 1 || skill.Metadata.Difficulty > 10 {
		return fmt.Errorf("difficulty must be between 1 and 10")
	}

	if skill.Metadata.MarketDemand < 0 || skill.Metadata.MarketDemand > 1 {
		return fmt.Errorf("market demand must be between 0 and 1")
	}

	return nil
}

// GetSkillStatistics returns various statistics about skills
func (s *SkillService) GetSkillStatistics(ctx context.Context) (*SkillStatistics, error) {
	totalActive, _, err := s.repo.List(ctx, repository.ListOptions{
		ActiveOnly: true,
		Limit:      0, // Get count only
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get active skills count: %w", err)
	}

	totalInactive, _, err := s.repo.List(ctx, repository.ListOptions{
		ActiveOnly: false,
		Limit:      0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get total skills count: %w", err)
	}

	mostUsed, err := s.repo.GetMostUsed(ctx, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to get most used skills: %w", err)
	}

	trending, _, err := s.repo.List(ctx, repository.ListOptions{
		ActiveOnly: true,
		Trending:   boolPtr(true),
		Limit:      10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get trending skills: %w", err)
	}

	return &SkillStatistics{
		TotalActive:   len(totalActive),
		TotalInactive: len(totalInactive) - len(totalActive),
		MostUsed:      mostUsed,
		Trending:      trending,
	}, nil
}

// SkillStatistics contains statistical information about skills
type SkillStatistics struct {
	TotalActive   int             `json:"total_active"`
	TotalInactive int             `json:"total_inactive"`
	MostUsed      []*models.Skill `json:"most_used"`
	Trending      []*models.Skill `json:"trending"`
}

// Helper function to create a boolean pointer
func boolPtr(b bool) *bool {
	return &b
}
