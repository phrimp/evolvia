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

	now := time.Now()
	userSkill.BloomsAssessment = models.BloomsTaxonomyAssessment{
		Remember:    0.0,
		Understand:  0.0,
		Apply:       0.0,
		Analyze:     0.0,
		Evaluate:    0.0,
		Create:      0.0,
		LastUpdated: now,
	}

	log.Printf("Initializing new user skill with zero Bloom's assessment for user %s, skill %s",
		userSkill.UserID.Hex(), userSkill.SkillID.Hex())

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

func (s *UserSkillService) UpdateBloomsAssessment(ctx context.Context, userID, skillID bson.ObjectID, assessment *models.BloomsTaxonomyAssessment) error {
	// Validate assessment scores (0-100)
	if err := s.validateBloomsAssessment(assessment); err != nil {
		return fmt.Errorf("invalid Bloom's assessment: %w", err)
	}

	// Verify user skill exists
	existing, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("user skill not found")
	}

	return s.userSkillRepo.UpdateBloomsAssessment(ctx, userID, skillID, assessment)
}

func (s *UserSkillService) GetBloomsAssessment(ctx context.Context, userID, skillID bson.ObjectID) (*models.BloomsTaxonomyAssessment, error) {
	userSkill, err := s.userSkillRepo.GetUserSkillWithBlooms(ctx, userID, skillID)
	if err != nil {
		return nil, err
	}
	if userSkill == nil {
		return nil, fmt.Errorf("user skill not found")
	}

	// Handle case where Bloom's assessment might not be initialized (legacy data)
	assessment := &userSkill.BloomsAssessment

	if assessment.LastUpdated.IsZero() {
		assessment = &models.BloomsTaxonomyAssessment{
			Remember:    0.0,
			Understand:  0.0,
			Apply:       0.0,
			Analyze:     0.0,
			Evaluate:    0.0,
			Create:      0.0,
			LastUpdated: time.Now(),
		}

		log.Printf("Legacy user skill found without Bloom's assessment, returning zeros for user %s, skill %s",
			userID.Hex(), skillID.Hex())
	}

	return assessment, nil
}

// GetBloomsAnalytics returns aggregated Bloom's data for a user
func (s *UserSkillService) GetBloomsAnalytics(ctx context.Context, userID bson.ObjectID) (*repository.BloomsAnalytics, error) {
	return s.userSkillRepo.GetBloomsAnalytics(ctx, userID)
}

// CalculateSkillLevelFromBlooms determines skill level based on Bloom's assessment
func (s *UserSkillService) CalculateSkillLevelFromBlooms(assessment *models.BloomsTaxonomyAssessment) models.SkillLevel {
	overallScore := assessment.GetOverallScore()

	// Define thresholds for skill levels based on Bloom's taxonomy
	switch {
	case overallScore >= 80 && assessment.Create >= 70:
		return models.SkillLevelExpert
	case overallScore >= 65 && assessment.Evaluate >= 60:
		return models.SkillLevelAdvanced
	case overallScore >= 50 && assessment.Apply >= 60:
		return models.SkillLevelIntermediate
	case overallScore >= 30:
		return models.SkillLevelBeginner
	default:
		return models.SkillLevelBeginner
	}
}

func (s *UserSkillService) GetRecommendedFocusArea(ctx context.Context, userID, skillID bson.ObjectID) (string, error) {
	assessment, err := s.GetBloomsAssessment(ctx, userID, skillID)
	if err != nil {
		return "", err
	}

	// For completely unassessed skills, start with fundamentals
	if assessment.GetOverallScore() == 0.0 {
		return "remember", nil
	}

	// Find the weakest area that should be developed next
	weakestArea := assessment.GetWeakestArea()

	// If no weak areas (all are assessed), recommend next logical progression
	if weakestArea == "" {
		// Follow Bloom's hierarchy - recommend next level if current is strong
		if assessment.Remember >= 70 && assessment.Understand < 70 {
			return "understand", nil
		}
		if assessment.Understand >= 70 && assessment.Apply < 70 {
			return "apply", nil
		}
		if assessment.Apply >= 70 && assessment.Analyze < 70 {
			return "analyze", nil
		}
		if assessment.Analyze >= 70 && assessment.Evaluate < 70 {
			return "evaluate", nil
		}
		if assessment.Evaluate >= 70 && assessment.Create < 70 {
			return "create", nil
		}

		// If all levels are strong, recommend maintaining current level
		return assessment.GetPrimaryStrength(), nil
	}

	return weakestArea, nil
}

// GetUsersWithBloomsExpertise finds users with high proficiency in specific Bloom's level
func (s *UserSkillService) GetUsersWithBloomsExpertise(ctx context.Context, skillID bson.ObjectID, bloomsLevel string, minScore float64, limit int) ([]*models.UserSkill, error) {
	// Validate Bloom's level
	validLevels := []string{"remember", "understand", "apply", "analyze", "evaluate", "create"}
	isValid := false
	for _, level := range validLevels {
		if level == bloomsLevel {
			isValid = true
			break
		}
	}
	if !isValid {
		return nil, fmt.Errorf("invalid Bloom's level: %s", bloomsLevel)
	}

	// Verify skill exists
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify skill existence: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	if limit <= 0 {
		limit = 10
	}

	return s.userSkillRepo.GetUsersWithBloomsLevel(ctx, skillID, bloomsLevel, minScore, limit)
}

// validateBloomsAssessment ensures all scores are within valid range
func (s *UserSkillService) validateBloomsAssessment(assessment *models.BloomsTaxonomyAssessment) error {
	if assessment == nil {
		return fmt.Errorf("assessment cannot be nil")
	}

	scores := map[string]float64{
		"remember":   assessment.Remember,
		"understand": assessment.Understand,
		"apply":      assessment.Apply,
		"analyze":    assessment.Analyze,
		"evaluate":   assessment.Evaluate,
		"create":     assessment.Create,
	}

	for level, score := range scores {
		if score < 0 || score > 100 {
			return fmt.Errorf("%s score must be between 0 and 100, got: %.2f", level, score)
		}
	}

	return nil
}

// Auto-update skill level based on Bloom's assessment (optional helper)
func (s *UserSkillService) UpdateSkillLevelFromBlooms(ctx context.Context, userID, skillID bson.ObjectID) error {
	assessment, err := s.GetBloomsAssessment(ctx, userID, skillID)
	if err != nil {
		return err
	}

	newLevel := s.CalculateSkillLevelFromBlooms(assessment)

	// Update the user skill level
	updates := &UserSkillUpdate{
		Level: newLevel,
	}

	_, err = s.UpdateUserSkill(ctx, userID, skillID, updates)
	return err
}

func (s *UserSkillService) HasBloomsAssessment(ctx context.Context, userID, skillID bson.ObjectID) (bool, error) {
	assessment, err := s.GetBloomsAssessment(ctx, userID, skillID)
	if err != nil {
		return false, err
	}

	// Check if any score is greater than 0 or if last_updated is not zero
	hasData := assessment.Remember > 0 || assessment.Understand > 0 || assessment.Apply > 0 ||
		assessment.Analyze > 0 || assessment.Evaluate > 0 || assessment.Create > 0 ||
		!assessment.LastUpdated.IsZero()

	return hasData, nil
}

func (s *UserSkillService) GetUserSkillsWithDetails(ctx context.Context, userID bson.ObjectID, opts repository.UserSkillListOptions) ([]*models.UserSkillWithDetails, error) {
	userSkills, err := s.userSkillRepo.GetByUser(ctx, userID, opts)
	if err != nil {
		return nil, err
	}

	var userSkillsWithDetails []*models.UserSkillWithDetails
	for _, userSkill := range userSkills {
		// Fetch skill details for each user skill
		skill, err := s.skillRepo.GetByID(ctx, userSkill.SkillID)
		if err != nil {
			log.Printf("Failed to get skill details for skill %s: %v", userSkill.SkillID.Hex(), err)
			continue // Skip this skill if we can't get details
		}
		if skill == nil {
			log.Printf("Skill not found for ID %s", userSkill.SkillID.Hex())
			continue
		}

		userSkillWithDetails := &models.UserSkillWithDetails{
			UserSkill:        userSkill,
			SkillName:        skill.Name,
			SkillDescription: skill.Description,
			SkillTags:        skill.Tags,
		}
		userSkillsWithDetails = append(userSkillsWithDetails, userSkillWithDetails)
	}

	return userSkillsWithDetails, nil
}

func (s *UserSkillService) GetUserSkillWithDetails(ctx context.Context, userID, skillID bson.ObjectID) (*models.UserSkillWithDetails, error) {
	userSkill, err := s.userSkillRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		return nil, err
	}
	if userSkill == nil {
		return nil, fmt.Errorf("user skill not found")
	}

	// Fetch skill details
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill details: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Create enhanced response
	userSkillWithDetails := &models.UserSkillWithDetails{
		UserSkill:        userSkill,
		SkillName:        skill.Name,
		SkillDescription: skill.Description,
		SkillTags:        skill.Tags,
	}

	return userSkillWithDetails, nil
}
