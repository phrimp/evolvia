package services

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserSkillService struct {
	userSkillRepo *repository.UserSkillRepository
	skillRepo     *repository.SkillRepository
}

// NewUserSkillService creates a new user skill service
func NewUserSkillService(userSkillRepo *repository.UserSkillRepository, skillRepo *repository.SkillRepository) (*UserSkillService, error) {
	service := &UserSkillService{
		userSkillRepo: userSkillRepo,
		skillRepo:     skillRepo,
	}

	// Initialize indexes
	if err := service.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize user skill service: %w", err)
	}

	return service, nil
}

// initialize sets up indexes
func (s *UserSkillService) initialize(ctx context.Context) error {
	log.Println("Initializing User Skill Service...")

	// Create database indexes
	log.Println("Creating user skill database indexes...")
	if err := s.userSkillRepo.InitializeIndexes(ctx); err != nil {
		return fmt.Errorf("failed to initialize indexes: %w", err)
	}
	log.Println("User skill database indexes created successfully")

	return nil
}

// AddUserSkill adds a skill to a user's profile
func (s *UserSkillService) AddUserSkill(ctx context.Context, userSkill *models.UserSkill) (*models.UserSkill, error) {
	// Validate input
	if err := s.validateUserSkill(userSkill); err != nil {
		return nil, fmt.Errorf("user skill validation failed: %w", err)
	}

	// Check if skill exists
	skill, err := s.skillRepo.GetByID(ctx, userSkill.SkillID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify skill existence: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Check if user already has this skill
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userSkill.UserID, userSkill.SkillID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user skill: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user already has this skill")
	}

	return s.userSkillRepo.Create(ctx, userSkill)
}

// GetUserSkill retrieves a specific user skill
func (s *UserSkillService) GetUserSkill(ctx context.Context, userID, skillID bson.ObjectID) (*models.UserSkill, error) {
	userSkill, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return nil, err
	}
	if userSkill == nil {
		return nil, fmt.Errorf("user skill not found")
	}

	return userSkill, nil
}

// GetUserSkills retrieves all skills for a user
func (s *UserSkillService) GetUserSkills(ctx context.Context, userID bson.ObjectID, opts repository.UserSkillListOptions) ([]*models.UserSkill, error) {
	return s.userSkillRepo.GetByUser(ctx, userID, opts)
}

// GetUsersWithSkill retrieves all users who have a specific skill
func (s *UserSkillService) GetUsersWithSkill(ctx context.Context, skillID bson.ObjectID, opts repository.UserSkillListOptions) ([]*models.UserSkill, error) {
	// Verify skill exists
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify skill existence: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	return s.userSkillRepo.GetBySkill(ctx, skillID, opts)
}

// UpdateUserSkill updates an existing user skill
func (s *UserSkillService) UpdateUserSkill(ctx context.Context, userID, skillID bson.ObjectID, updates *UserSkillUpdate) (*models.UserSkill, error) {
	// Get existing user skill
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("user skill not found")
	}

	// Apply updates
	if updates.Level != "" {
		existing.Level = updates.Level
	}
	if updates.Confidence != nil {
		existing.Confidence = *updates.Confidence
	}
	if updates.YearsExperience != nil {
		existing.YearsExperience = *updates.YearsExperience
	}
	if updates.LastUsed != nil {
		existing.LastUsed = updates.LastUsed
	}

	// Validate updated skill
	if err := s.validateUserSkill(existing); err != nil {
		return nil, fmt.Errorf("user skill validation failed: %w", err)
	}

	return s.userSkillRepo.Update(ctx, existing.ID, existing)
}

// RemoveUserSkill removes a skill from a user's profile
func (s *UserSkillService) RemoveUserSkill(ctx context.Context, userID, skillID bson.ObjectID) error {
	// Check if user skill exists
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("user skill not found")
	}

	return s.userSkillRepo.DeleteByUserAndSkill(ctx, userID, skillID)
}

// UpdateLastUsed updates when a user last used a skill
func (s *UserSkillService) UpdateLastUsed(ctx context.Context, userID, skillID bson.ObjectID) error {
	// Verify user skill exists
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("user skill not found")
	}

	return s.userSkillRepo.UpdateLastUsed(ctx, userID, skillID)
}

// EndorseUserSkill adds an endorsement to a user's skill
func (s *UserSkillService) EndorseUserSkill(ctx context.Context, userID, skillID bson.ObjectID) error {
	// Verify user skill exists
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("user skill not found")
	}

	return s.userSkillRepo.IncrementEndorsements(ctx, userID, skillID)
}

// VerifyUserSkill marks a user skill as verified
func (s *UserSkillService) VerifyUserSkill(ctx context.Context, userID, skillID bson.ObjectID, verified bool) error {
	// Verify user skill exists
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("user skill not found")
	}

	return s.userSkillRepo.SetVerified(ctx, userID, skillID, verified)
}

// GetTopUsersForSkill retrieves users with highest proficiency in a skill
func (s *UserSkillService) GetTopUsersForSkill(ctx context.Context, skillID bson.ObjectID, limit int) ([]*models.UserSkill, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Verify skill exists
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify skill existence: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	return s.userSkillRepo.GetTopSkillsByLevel(ctx, skillID, limit)
}

// GetUserSkillMatrix retrieves a user's skill profile organized by categories
func (s *UserSkillService) GetUserSkillMatrix(ctx context.Context, userID bson.ObjectID) (*UserSkillMatrix, error) {
	// Get all user skills
	userSkills, err := s.userSkillRepo.GetByUser(ctx, userID, repository.UserSkillListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user skills: %w", err)
	}

	// Group by level
	matrix := &UserSkillMatrix{
		UserID:     userID,
		ByLevel:    make(map[models.SkillLevel][]*models.UserSkill),
		Total:      len(userSkills),
		Verified:   0,
		LastUpdate: time.Now(),
	}

	for _, userSkill := range userSkills {
		matrix.ByLevel[userSkill.Level] = append(matrix.ByLevel[userSkill.Level], userSkill)
		if userSkill.Verified {
			matrix.Verified++
		}
	}

	return matrix, nil
}

// GetSkillGaps identifies missing prerequisite skills for a user
func (s *UserSkillService) GetSkillGaps(ctx context.Context, userID bson.ObjectID, targetSkillID bson.ObjectID) ([]*models.Skill, error) {
	// Get target skill
	targetSkill, err := s.skillRepo.GetByID(ctx, targetSkillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target skill: %w", err)
	}
	if targetSkill == nil {
		return nil, fmt.Errorf("target skill not found")
	}

	// Get user's current skills
	userSkills, err := s.userSkillRepo.GetByUser(ctx, userID, repository.UserSkillListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user skills: %w", err)
	}

	// Create map of user's skill IDs
	userSkillMap := make(map[bson.ObjectID]bool)
	for _, userSkill := range userSkills {
		userSkillMap[userSkill.SkillID] = true
	}

	// Find missing prerequisites
	var gaps []*models.Skill
	for _, relation := range targetSkill.Relations {
		if relation.RelationType == models.RelationPrerequisite {
			if !userSkillMap[relation.SkillID] {
				skill, err := s.skillRepo.GetByID(ctx, relation.SkillID)
				if err != nil {
					log.Printf("Failed to get prerequisite skill %s: %v", relation.SkillID.Hex(), err)
					continue
				}
				if skill != nil {
					gaps = append(gaps, skill)
				}
			}
		}
	}

	return gaps, nil
}

// BatchAddUserSkills adds multiple skills to a user's profile
func (s *UserSkillService) BatchAddUserSkills(ctx context.Context, userSkills []*models.UserSkill) error {
	// Validate all user skills
	for i, userSkill := range userSkills {
		if err := s.validateUserSkill(userSkill); err != nil {
			return fmt.Errorf("user skill validation failed at index %d: %w", i, err)
		}
	}

	// Check for duplicates within the batch
	seen := make(map[string]bool)
	for i, userSkill := range userSkills {
		key := fmt.Sprintf("%s-%s", userSkill.UserID.Hex(), userSkill.SkillID.Hex())
		if seen[key] {
			return fmt.Errorf("duplicate user-skill combination at index %d", i)
		}
		seen[key] = true
	}

	// Check for existing user skills
	for i, userSkill := range userSkills {
		existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userSkill.UserID, userSkill.SkillID)
		if err != nil {
			return fmt.Errorf("failed to check existing user skill at index %d: %w", i, err)
		}
		if existing != nil {
			return fmt.Errorf("user skill already exists at index %d", i)
		}
	}

	return s.userSkillRepo.BatchCreate(ctx, userSkills)
}

// validateUserSkill performs validation on user skill data
func (s *UserSkillService) validateUserSkill(userSkill *models.UserSkill) error {
	if userSkill == nil {
		return fmt.Errorf("user skill cannot be nil")
	}

	if userSkill.UserID.IsZero() {
		return fmt.Errorf("user ID is required")
	}

	if userSkill.SkillID.IsZero() {
		return fmt.Errorf("skill ID is required")
	}

	if userSkill.Level == "" {
		return fmt.Errorf("skill level is required")
	}

	// Validate skill level
	switch userSkill.Level {
	case models.SkillLevelBeginner, models.SkillLevelIntermediate, models.SkillLevelAdvanced, models.SkillLevelExpert:
		// Valid levels
	default:
		return fmt.Errorf("invalid skill level: %s", userSkill.Level)
	}

	if userSkill.Confidence < 0 || userSkill.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	if userSkill.YearsExperience < 0 {
		return fmt.Errorf("years of experience cannot be negative")
	}

	if userSkill.Endorsements < 0 {
		return fmt.Errorf("endorsements cannot be negative")
	}

	return nil
}

// UserSkillUpdate represents fields that can be updated for a user skill
type UserSkillUpdate struct {
	Level           models.SkillLevel `json:"level,omitempty"`
	Confidence      *float64          `json:"confidence,omitempty"`
	YearsExperience *int              `json:"years_experience,omitempty"`
	LastUsed        *time.Time        `json:"last_used,omitempty"`
}

// UserSkillMatrix represents a user's skill profile organized by categories
type UserSkillMatrix struct {
	UserID     bson.ObjectID                             `json:"user_id"`
	ByLevel    map[models.SkillLevel][]*models.UserSkill `json:"by_level"`
	Total      int                                       `json:"total"`
	Verified   int                                       `json:"verified"`
	LastUpdate time.Time                                 `json:"last_update"`
}
