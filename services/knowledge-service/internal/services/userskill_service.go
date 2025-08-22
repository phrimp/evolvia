package services

import (
	"context"
	"fmt"
	"knowledge-service/internal/models"
	"knowledge-service/internal/repository"
	"log"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserSkillService struct {
	userSkillRepo                *repository.UserSkillRepository
	skillVerificationHistoryRepo *repository.SkillVerificationHistoryRepository
	skillRepo                    *repository.SkillRepository
}

// NewUserSkillService creates a new user skill service
func NewUserSkillService(userSkillRepo *repository.UserSkillRepository, skillRepo *repository.SkillRepository, skillVerificationHistoryRepo *repository.SkillVerificationHistoryRepository) (*UserSkillService, error) {
	service := &UserSkillService{
		userSkillRepo:                userSkillRepo,
		skillRepo:                    skillRepo,
		skillVerificationHistoryRepo: skillVerificationHistoryRepo,
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
	if !skill.Addable {
		return nil, fmt.Errorf("this skill cannot be added")
	}

	for _, rel := range skill.Relations {
		if rel.RelationType == models.RelationPrerequisite {
			prereqSkill, err := s.userSkillRepo.GetByUserAndSkill(ctx, userSkill.UserID, rel.SkillID)
			if err != nil {
				return nil, fmt.Errorf("failed to check prerequisite skill: %w", err)
			}
			if prereqSkill == nil {
				return nil, fmt.Errorf("prerequisite skill is missing")
			}
			if prereqSkill.Level != models.SkillLevelIntermediate {
				return nil, fmt.Errorf("prerequisite skill must be at intermediate level")
			}
		}
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
		Verified:    false,
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

	err = s.userSkillRepo.UpdateBloomsAssessment(ctx, userID, skillID, assessment)
	if err != nil {
		return err
	}
	s.skillVerificationHistoryRepo.Create(ctx, &models.SkillProgressHistory{
		UserID:            userID,
		SkillID:           skillID,
		BloomsSnapshot:    *assessment,
		TotalHours:        0,
		VerificationCount: 1,
		TriggerEvent:      "self-assessment",
	})

	return nil
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
	isValid := slices.Contains(validLevels, bloomsLevel)
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

// GetAggregatedSkillAssessment calculates hybrid skill assessment from builds_on relationships + own verification history
// Uses weight distribution: builds_on skills contribute their defined weights, skill's own history fills remaining weight
func (s *UserSkillService) GetAggregatedSkillAssessment(ctx context.Context, userID, skillID bson.ObjectID) (*models.AggregatedSkillAssessment, error) {
	// Get the target skill to find its "builds_on" relationships
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Find all "builds_on" relationships
	var buildsOnRelations []models.SkillRelation
	var buildsOnTotalWeight float64
	for _, relation := range skill.Relations {
		if relation.RelationType == models.RelationBuildsOn {
			buildsOnRelations = append(buildsOnRelations, relation)
			buildsOnTotalWeight += relation.Strength
		}
	}

	if len(buildsOnRelations) == 0 {
		return nil, fmt.Errorf("skill has no builds_on relationships")
	}

	// Calculate remaining weight for skill's own verification history
	ownWeight := max(1.0-buildsOnTotalWeight, 0)

	// Extract skill IDs for batch retrieval
	var relatedSkillIDs []bson.ObjectID
	for _, relation := range buildsOnRelations {
		relatedSkillIDs = append(relatedSkillIDs, relation.SkillID)
	}

	// Get verification histories for all related skills
	historiesMap, err := s.skillVerificationHistoryRepo.GetByUserAndSkills(ctx, userID, relatedSkillIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get verification histories: %w", err)
	}

	// Get skill's own verification history
	ownHistory, err := s.skillVerificationHistoryRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		// Log error but continue - own history is optional
		ownHistory = nil
	}

	// Build weighted skill histories (builds_on skills)
	var weightedSkills []*models.WeightedSkillHistory
	var totalWeightVerified float64
	var overallScore float64
	minSkillProgress := 100.0 // Track minimum progress for capping at 100%
	var hasVerificationData bool

	for _, relation := range buildsOnRelations {
		// Get skill details for name
		relatedSkill, err := s.skillRepo.GetByID(ctx, relation.SkillID)
		if err != nil {
			continue // Skip if can't get skill details
		}
		if relatedSkill == nil {
			continue
		}

		weightedSkill := &models.WeightedSkillHistory{
			SkillID:        relation.SkillID,
			SkillName:      relatedSkill.Name,
			RelationWeight: relation.Strength,
			History:        historiesMap[relation.SkillID],
		}

		// Calculate contribution if there's verification history
		if len(weightedSkill.History) > 0 {
			// Use most recent assessment for calculation
			latestHistory := weightedSkill.History[0] // Already sorted by timestamp desc
			weightedSkill.LatestAssessment = &latestHistory.BloomsSnapshot
			latestScore := latestHistory.BloomsSnapshot.GetOverallScore()
			weightedSkill.Contribution = relation.Strength * latestScore

			totalWeightVerified += relation.Strength
			overallScore += weightedSkill.Contribution
			hasVerificationData = true

			// Track minimum skill progress for 100% requirement logic
			if latestScore < minSkillProgress {
				minSkillProgress = latestScore
			}
		}

		weightedSkills = append(weightedSkills, weightedSkill)
	}

	// Add skill's own verification history if it exists and has remaining weight
	if ownWeight > 0 && len(ownHistory) > 0 {
		// Create a weighted skill entry for the skill itself
		ownWeightedSkill := &models.WeightedSkillHistory{
			SkillID:        skillID,
			SkillName:      skill.Name,
			RelationWeight: ownWeight,
			History:        ownHistory,
		}

		// Use most recent assessment for calculation
		latestOwnHistory := ownHistory[0] // Already sorted by timestamp desc
		ownWeightedSkill.LatestAssessment = &latestOwnHistory.BloomsSnapshot
		ownLatestScore := latestOwnHistory.BloomsSnapshot.GetOverallScore()
		ownWeightedSkill.Contribution = ownWeight * ownLatestScore

		totalWeightVerified += ownWeight
		overallScore += ownWeightedSkill.Contribution
		hasVerificationData = true

		// Track minimum skill progress for 100% requirement logic
		if ownLatestScore < minSkillProgress {
			minSkillProgress = ownLatestScore
		}

		weightedSkills = append(weightedSkills, ownWeightedSkill)
	}

	// Apply constraint: 100% total progress only when all contributing skills are 100%
	// If any skill is below 100%, cap the overall score accordingly
	if hasVerificationData && minSkillProgress < 100.0 {
		// Scale down overall score proportionally to the weakest skill
		scalingFactor := minSkillProgress / 100.0
		overallScore = overallScore * scalingFactor
	}

	// Create aggregated assessment
	assessment := &models.AggregatedSkillAssessment{
		SkillID:             skillID,
		SkillName:           skill.Name,
		UserID:              userID,
		OverallScore:        overallScore,
		WeightedSkills:      weightedSkills,
		TotalWeightVerified: totalWeightVerified,
		LastCalculated:      time.Now(),
		IsComplete:          totalWeightVerified >= 0.699, // Allow small floating point tolerance (lam tron)
	}

	return assessment, nil
}

// CreateAggregatedSkillHistory creates a verification history record for aggregated skill calculation
func (s *UserSkillService) CreateAggregatedSkillHistory(ctx context.Context, userID, skillID bson.ObjectID) (*models.SkillProgressHistory, error) {
	// Get aggregated assessment
	assessment, err := s.GetAggregatedSkillAssessment(ctx, userID, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregated assessment: %w", err)
	}

	// Calculate aggregated Bloom's scores
	var aggregatedBlooms models.BloomsTaxonomyAssessment
	totalWeight := 0.0

	for _, weightedSkill := range assessment.WeightedSkills {
		if weightedSkill.LatestAssessment != nil {
			weight := weightedSkill.RelationWeight
			blooms := weightedSkill.LatestAssessment

			aggregatedBlooms.Remember += blooms.Remember * weight
			aggregatedBlooms.Understand += blooms.Understand * weight
			aggregatedBlooms.Apply += blooms.Apply * weight
			aggregatedBlooms.Analyze += blooms.Analyze * weight
			aggregatedBlooms.Evaluate += blooms.Evaluate * weight
			aggregatedBlooms.Create += blooms.Create * weight

			totalWeight += weight
		}
	}

	// Normalize if partial verification history exists
	if totalWeight > 0 && totalWeight < 1.0 {
		factor := 1.0 / totalWeight
		aggregatedBlooms.Remember *= factor
		aggregatedBlooms.Understand *= factor
		aggregatedBlooms.Apply *= factor
		aggregatedBlooms.Analyze *= factor
		aggregatedBlooms.Evaluate *= factor
		aggregatedBlooms.Create *= factor
	}

	aggregatedBlooms.Verified = assessment.IsComplete
	aggregatedBlooms.LastUpdated = time.Now()

	// Create verification history record
	history := &models.SkillProgressHistory{
		UserID:            userID,
		SkillID:           skillID,
		BloomsSnapshot:    aggregatedBlooms,
		TotalHours:        0, // Not applicable for aggregated skills
		VerificationCount: 1,
		Timestamp:         time.Now(),
		TriggerEvent:      "aggregated_calculation",
		OverallScore:      assessment.OverallScore,
		IsAggregated:      true,
	}

	return s.skillVerificationHistoryRepo.Create(ctx, history)
}

// GetLatestPerLevelComposite creates a composite assessment using the latest verified score
// for each individual Bloom's level from across all historical records
func (s *UserSkillService) GetLatestPerLevelComposite(ctx context.Context, userID, skillID bson.ObjectID) (*models.BloomsTaxonomyAssessment, error) {
	// Get ALL verification history for this skill (sorted by timestamp DESC)
	allHistory, err := s.skillVerificationHistoryRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil || len(allHistory) == 0 {
		return &models.BloomsTaxonomyAssessment{}, nil
	}

	// Initialize composite assessment with zero values
	composite := &models.BloomsTaxonomyAssessment{}

	// Track latest timestamp for each Bloom's level
	levelTimestamps := map[string]time.Time{
		"remember":   {},
		"understand": {},
		"apply":      {},
		"analyze":    {},
		"evaluate":   {},
		"create":     {},
	}

	// Process history records to find latest verified score for each level
	for _, record := range allHistory {
		if !record.BloomsSnapshot.Verified {
			continue // Skip unverified assessments
		}

		ts := record.Timestamp
		snapshot := record.BloomsSnapshot

		// Update each Bloom's level if this is the latest verified assessment for that level
		if levelTimestamps["remember"].IsZero() || ts.After(levelTimestamps["remember"]) {
			composite.Remember = snapshot.Remember
			levelTimestamps["remember"] = ts
		}
		if levelTimestamps["understand"].IsZero() || ts.After(levelTimestamps["understand"]) {
			composite.Understand = snapshot.Understand
			levelTimestamps["understand"] = ts
		}
		if levelTimestamps["apply"].IsZero() || ts.After(levelTimestamps["apply"]) {
			composite.Apply = snapshot.Apply
			levelTimestamps["apply"] = ts
		}
		if levelTimestamps["analyze"].IsZero() || ts.After(levelTimestamps["analyze"]) {
			composite.Analyze = snapshot.Analyze
			levelTimestamps["analyze"] = ts
		}
		if levelTimestamps["evaluate"].IsZero() || ts.After(levelTimestamps["evaluate"]) {
			composite.Evaluate = snapshot.Evaluate
			levelTimestamps["evaluate"] = ts
		}
		if levelTimestamps["create"].IsZero() || ts.After(levelTimestamps["create"]) {
			composite.Create = snapshot.Create
			levelTimestamps["create"] = ts
		}
	}

	composite.Verified = true
	composite.LastUpdated = time.Now()
	return composite, nil
}

// GetSkillAssessmentWithAggregation gets skill assessment with the following priority:
// 1. Hybrid verification history (builds_on + own verification with weight distribution)
// 2. Direct self-assessment (fallback only if no verification history exists anywhere)
// 3. Zero assessment if no data available
func (s *UserSkillService) GetSkillAssessmentWithAggregation(ctx context.Context, userID, skillID bson.ObjectID) (*models.BloomsTaxonomyAssessment, error) {
	// Check if skill has builds_on relationships
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Check for builds_on relationships
	hasBuildsOn := false
	for _, relation := range skill.Relations {
		if relation.RelationType == models.RelationBuildsOn {
			hasBuildsOn = true
			break
		}
	}

	// First priority: Try hybrid verification history approach
	if hasBuildsOn {
		aggregatedAssessment, err := s.GetAggregatedSkillAssessment(ctx, userID, skillID)
		if err == nil && aggregatedAssessment != nil && aggregatedAssessment.TotalWeightVerified > 0 {
			// Calculate hybrid assessment from verification history (builds_on + own)
			return s.calculateAggregatedBloomsFromHistory(aggregatedAssessment), nil
		}
	} else {
		// For skills without builds_on relationships, use latest-per-level composite
		return s.GetLatestPerLevelComposite(ctx, userID, skillID)
	}

	// Second priority: Use self-assessment only if NO verification history exists
	directAssessment, err := s.GetBloomsAssessment(ctx, userID, skillID)
	if err == nil && directAssessment != nil && directAssessment.GetOverallScore() > 0 {
		return directAssessment, nil
	}

	// Return zero assessment if no data available
	return &models.BloomsTaxonomyAssessment{}, nil
}

// calculateAggregatedBloomsFromHistory calculates Bloom's taxonomy scores from aggregated assessment
// using latest-per-level composite for each weighted skill
func (s *UserSkillService) calculateAggregatedBloomsFromHistory(aggregatedAssessment *models.AggregatedSkillAssessment) *models.BloomsTaxonomyAssessment {
	var blooms models.BloomsTaxonomyAssessment
	totalWeight := 0.0

	for _, weightedSkill := range aggregatedAssessment.WeightedSkills {
		if len(weightedSkill.History) > 0 {
			weight := weightedSkill.RelationWeight

			// Use latest-per-level composite for this weighted skill
			// Extract userID from the first history record
			userID := weightedSkill.History[0].UserID
			skillID := weightedSkill.SkillID

			// Get composite assessment for this skill
			compositeAssessment, err := s.GetLatestPerLevelComposite(context.Background(), userID, skillID)
			if err == nil && compositeAssessment != nil {
				blooms.Remember += compositeAssessment.Remember * weight
				blooms.Understand += compositeAssessment.Understand * weight
				blooms.Apply += compositeAssessment.Apply * weight
				blooms.Analyze += compositeAssessment.Analyze * weight
				blooms.Evaluate += compositeAssessment.Evaluate * weight
				blooms.Create += compositeAssessment.Create * weight

				totalWeight += weight
			}
		}
	}

	// Normalize if only partial verification history exists
	if totalWeight > 0 && totalWeight < 1.0 {
		factor := 1.0 / totalWeight
		blooms.Remember *= factor
		blooms.Understand *= factor
		blooms.Apply *= factor
		blooms.Analyze *= factor
		blooms.Evaluate *= factor
		blooms.Create *= factor
	}

	blooms.Verified = aggregatedAssessment.IsComplete
	blooms.LastUpdated = time.Now()

	return &blooms
}

// GetComprehensiveVerificationHistory gets complete verification history including builds_on skills and time tracking
func (s *UserSkillService) GetComprehensiveVerificationHistory(ctx context.Context, userID, skillID bson.ObjectID) (*models.ComprehensiveVerificationHistory, error) {
	// Get the target skill to find its "builds_on" relationships
	skill, err := s.skillRepo.GetByID(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	if skill == nil {
		return nil, fmt.Errorf("skill not found")
	}

	// Get own skill verification history
	ownHistory, err := s.skillVerificationHistoryRepo.GetByUserAndSkill(ctx, userID, skillID)
	if err != nil {
		ownHistory = nil // Continue even if no own history
	}

	// Calculate total time spent on own skill
	var ownTotalHours float64
	for _, history := range ownHistory {
		ownTotalHours += history.TotalHours
	}

	// Find all "builds_on" relationships
	var buildsOnRelations []models.SkillRelation
	for _, relation := range skill.Relations {
		if relation.RelationType == models.RelationBuildsOn {
			buildsOnRelations = append(buildsOnRelations, relation)
		}
	}

	// Get builds_on skill histories if they exist
	buildsOnHistories := make(map[bson.ObjectID]*models.SkillHistoryWithTime)
	var totalBuildsOnHours float64

	if len(buildsOnRelations) > 0 {
		var relatedSkillIDs []bson.ObjectID
		for _, relation := range buildsOnRelations {
			relatedSkillIDs = append(relatedSkillIDs, relation.SkillID)
		}

		// Get verification histories for all related skills
		historiesMap, err := s.skillVerificationHistoryRepo.GetByUserAndSkills(ctx, userID, relatedSkillIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get builds_on verification histories: %w", err)
		}

		// Process each builds_on skill
		for _, relation := range buildsOnRelations {
			relatedSkill, err := s.skillRepo.GetByID(ctx, relation.SkillID)
			if err != nil {
				continue // Skip if can't get skill details
			}
			if relatedSkill == nil {
				continue
			}

			history := historiesMap[relation.SkillID]
			var skillTotalHours float64

			// Calculate total hours for this builds_on skill
			for _, h := range history {
				skillTotalHours += h.TotalHours
			}
			totalBuildsOnHours += skillTotalHours

			buildsOnHistories[relation.SkillID] = &models.SkillHistoryWithTime{
				SkillID:        relation.SkillID,
				SkillName:      relatedSkill.Name,
				RelationWeight: relation.Strength,
				History:        history,
				TotalHours:     skillTotalHours,
			}
		}
	}

	// Create combined chronological timeline
	var combinedTimeline []*models.TimelineEntry

	// Add own skill history to timeline
	ownWeight := 1.0 - getTotalBuildsOnWeight(buildsOnRelations)
	for _, history := range ownHistory {
		entry := &models.TimelineEntry{
			Timestamp:      history.Timestamp,
			SkillID:        skillID,
			SkillName:      skill.Name,
			Hours:          history.TotalHours,
			BloomsSnapshot: history.BloomsSnapshot,
			TriggerEvent:   history.TriggerEvent,
			RelationWeight: ownWeight,
			IsOwnSkill:     true,
		}
		combinedTimeline = append(combinedTimeline, entry)
	}

	// Add builds_on skill histories to timeline
	for relatedSkillID, historyWithTime := range buildsOnHistories {
		for _, history := range historyWithTime.History {
			entry := &models.TimelineEntry{
				Timestamp:      history.Timestamp,
				SkillID:        relatedSkillID,
				SkillName:      historyWithTime.SkillName,
				Hours:          history.TotalHours,
				BloomsSnapshot: history.BloomsSnapshot,
				TriggerEvent:   history.TriggerEvent,
				RelationWeight: historyWithTime.RelationWeight,
				IsOwnSkill:     false,
			}
			combinedTimeline = append(combinedTimeline, entry)
		}
	}

	// Sort timeline by timestamp (most recent first)
	for i := 0; i < len(combinedTimeline)-1; i++ {
		for j := i + 1; j < len(combinedTimeline); j++ {
			if combinedTimeline[i].Timestamp.Before(combinedTimeline[j].Timestamp) {
				combinedTimeline[i], combinedTimeline[j] = combinedTimeline[j], combinedTimeline[i]
			}
		}
	}

	// Create own history with time
	ownHistoryWithTime := &models.SkillHistoryWithTime{
		SkillID:    skillID,
		SkillName:  skill.Name,
		History:    ownHistory,
		TotalHours: ownTotalHours,
	}
	if len(ownHistory) > 0 {
		ownHistoryWithTime.LatestAssessment = &ownHistory[0].BloomsSnapshot
	}

	// Convert buildsOnHistories map to slice
	var buildsOnHistorySlice []*models.SkillHistoryWithTime
	for _, historyWithTime := range buildsOnHistories {
		buildsOnHistorySlice = append(buildsOnHistorySlice, historyWithTime)
	}

	return &models.ComprehensiveVerificationHistory{
		UserID:          userID,
		SkillID:         skillID,
		SkillName:       skill.Name,
		OwnHistory:      ownHistoryWithTime,
		BuildsOnHistory: buildsOnHistorySlice,
		Timeline:        combinedTimeline,
		TotalHoursSpent: ownTotalHours + totalBuildsOnHours,
		GeneratedAt:     time.Now(),
	}, nil
}

// Helper function to calculate total builds_on weight
func getTotalBuildsOnWeight(relations []models.SkillRelation) float64 {
	var total float64
	for _, relation := range relations {
		if relation.RelationType == models.RelationBuildsOn {
			total += relation.Strength
		}
	}
	return total
}

// UserSkillProgressDetail represents detailed skill progress information for the API response
type UserSkillProgressDetail struct {
	SkillID          bson.ObjectID                   `json:"skill_id"`
	SkillName        string                          `json:"skill_name"`
	SkillDescription string                          `json:"skill_description,omitempty"`
	SkillLevel       models.SkillLevel               `json:"skill_level"`
	Progress         float64                         `json:"progress"` // Overall progress score 0-100
	BloomsAssessment models.BloomsTaxonomyAssessment `json:"blooms_assessment"`
	LatestHistory    *ProgressHistorySummary         `json:"latest_history,omitempty"`
	Confidence       float64                         `json:"confidence"`
	YearsExperience  int                             `json:"years_experience"`
	Verified         bool                            `json:"verified"`
	LastUsed         *time.Time                      `json:"last_used,omitempty"`
	UpdatedAt        time.Time                       `json:"updated_at"`
}

// ProgressHistorySummary represents summary of recent progress history
type ProgressHistorySummary struct {
	Timestamp      time.Time                       `json:"timestamp"`
	TotalHours     float64                         `json:"total_hours"`
	TriggerEvent   string                          `json:"trigger_event"`
	OverallScore   float64                         `json:"overall_score"`
	PreviousScore  float64                         `json:"previous_score,omitempty"`
	Improvement    float64                         `json:"improvement,omitempty"`
	BloomsSnapshot models.BloomsTaxonomyAssessment `json:"blooms_snapshot"`
}

// GetTopUserSkillProgress retrieves top user skill progress with comprehensive details
func (s *UserSkillService) GetTopUserSkillProgress(ctx context.Context, userID bson.ObjectID, criteria string, limit int) ([]*UserSkillProgressDetail, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	// Get all user skills for processing
	userSkills, err := s.userSkillRepo.GetByUser(ctx, userID, repository.UserSkillListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user skills: %w", err)
	}

	if len(userSkills) == 0 {
		return []*UserSkillProgressDetail{}, nil
	}

	// Build detailed progress information for each skill
	var progressDetails []*UserSkillProgressDetail
	for _, userSkill := range userSkills {
		// Get skill details
		skill, err := s.skillRepo.GetByID(ctx, userSkill.SkillID)
		if err != nil {
			log.Printf("Failed to get skill details for skill %s: %v", userSkill.SkillID.Hex(), err)
			continue
		}
		if skill == nil {
			continue
		}

		// Calculate overall progress using hybrid assessment
		bloomsAssessment, err := s.GetSkillAssessmentWithAggregation(ctx, userID, userSkill.SkillID)
		if err != nil {
			log.Printf("Failed to get blooms assessment for skill %s: %v", userSkill.SkillID.Hex(), err)
			// Use default empty assessment
			bloomsAssessment = &models.BloomsTaxonomyAssessment{}
		}

		progress := bloomsAssessment.GetOverallScore()

		// Get latest verification history for additional details
		var latestHistory *ProgressHistorySummary
		skillHistory, err := s.skillVerificationHistoryRepo.GetLatestByUserAndSkill(ctx, userID, userSkill.SkillID)
		if err == nil && skillHistory != nil {
			latestHistory = &ProgressHistorySummary{
				Timestamp:      skillHistory.Timestamp,
				TotalHours:     skillHistory.TotalHours,
				TriggerEvent:   skillHistory.TriggerEvent,
				OverallScore:   skillHistory.OverallScore,
				BloomsSnapshot: skillHistory.BloomsSnapshot,
			}

			// Calculate improvement from previous record if available
			allHistory, err := s.skillVerificationHistoryRepo.GetByUserAndSkill(ctx, userID, userSkill.SkillID)
			if err == nil && len(allHistory) > 1 {
				previousRecord := allHistory[1] // Second most recent
				latestHistory.PreviousScore = previousRecord.OverallScore
				latestHistory.Improvement = skillHistory.OverallScore - previousRecord.OverallScore
			}
		}

		progressDetail := &UserSkillProgressDetail{
			SkillID:          userSkill.SkillID,
			SkillName:        skill.Name,
			SkillDescription: skill.Description,
			SkillLevel:       userSkill.Level,
			Progress:         progress,
			BloomsAssessment: *bloomsAssessment,
			LatestHistory:    latestHistory,
			Confidence:       userSkill.Confidence,
			YearsExperience:  userSkill.YearsExperience,
			Verified:         userSkill.Verified,
			LastUsed:         userSkill.LastUsed,
			UpdatedAt:        userSkill.UpdatedAt,
		}

		progressDetails = append(progressDetails, progressDetail)
	}

	// Sort based on criteria
	switch criteria {
	case "recent_activity":
		// Sort by most recent activity (last_used or updated_at)
		for i := 0; i < len(progressDetails)-1; i++ {
			for j := i + 1; j < len(progressDetails); j++ {
				iTime := progressDetails[i].UpdatedAt
				if progressDetails[i].LastUsed != nil {
					iTime = *progressDetails[i].LastUsed
				}
				jTime := progressDetails[j].UpdatedAt
				if progressDetails[j].LastUsed != nil {
					jTime = *progressDetails[j].LastUsed
				}
				if iTime.Before(jTime) {
					progressDetails[i], progressDetails[j] = progressDetails[j], progressDetails[i]
				}
			}
		}
	case "improvement":
		// Sort by recent improvement (requires history)
		for i := 0; i < len(progressDetails)-1; i++ {
			for j := i + 1; j < len(progressDetails); j++ {
				iImprovement := 0.0
				jImprovement := 0.0
				if progressDetails[i].LatestHistory != nil {
					iImprovement = progressDetails[i].LatestHistory.Improvement
				}
				if progressDetails[j].LatestHistory != nil {
					jImprovement = progressDetails[j].LatestHistory.Improvement
				}
				if iImprovement < jImprovement {
					progressDetails[i], progressDetails[j] = progressDetails[j], progressDetails[i]
				}
			}
		}
	default: // "overall_progress" and fallback
		// Sort by overall progress score
		for i := 0; i < len(progressDetails)-1; i++ {
			for j := i + 1; j < len(progressDetails); j++ {
				if progressDetails[i].Progress < progressDetails[j].Progress {
					progressDetails[i], progressDetails[j] = progressDetails[j], progressDetails[i]
				}
			}
		}
	}

	// Return top N results
	if len(progressDetails) > limit {
		progressDetails = progressDetails[:limit]
	}

	return progressDetails, nil
}

// GetTotalPassedSkillsCount calculates total passed skills count based on IsComplete criteria (totalWeightVerified >= 0.699)
func (s *UserSkillService) GetTotalPassedSkillsCount(ctx context.Context, userID bson.ObjectID) (*models.TotalPassedSkillsResult, error) {
	// Get all user skills for this user
	userSkills, err := s.userSkillRepo.GetByUser(ctx, userID, repository.UserSkillListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get user skills: %w", err)
	}

	var totalPassedSkills int
	var totalAssessedSkills int
	const completionThreshold = 0.699

	// Iterate through user skills and check completion status
	for _, userSkill := range userSkills {
		// Check if skill has builds_on relationships for aggregated assessment
		skill, err := s.skillRepo.GetByID(ctx, userSkill.SkillID)
		if err != nil {
			log.Printf("Failed to get skill details for skill %s: %v", userSkill.SkillID.Hex(), err)
			continue // Skip this skill if we can't get details
		}
		if skill == nil {
			continue
		}

		// Check if skill has builds_on relationships
		hasBuildsOn := false
		for _, relation := range skill.Relations {
			if relation.RelationType == models.RelationBuildsOn {
				hasBuildsOn = true
				break
			}
		}

		// For skills with builds_on relationships, use aggregated assessment
		if hasBuildsOn {
			aggregatedAssessment, err := s.GetAggregatedSkillAssessment(ctx, userID, userSkill.SkillID)
			if err == nil && aggregatedAssessment != nil {
				totalAssessedSkills++
				if aggregatedAssessment.IsComplete {
					totalPassedSkills++
				}
			}
		} else {
			// For skills without builds_on relationships, check if they have verification history
			ownHistory, err := s.skillVerificationHistoryRepo.GetByUserAndSkill(ctx, userID, userSkill.SkillID)
			if err == nil && len(ownHistory) > 0 {
				totalAssessedSkills++
				// For skills without builds_on, consider them passed if they have any verification history
				// Since the completion logic is specifically for aggregated skills, we consider
				// standalone skills as passed if they have verification records
				totalPassedSkills++
			}
		}
	}

	// Calculate pass rate
	var passRate float64
	if totalAssessedSkills > 0 {
		passRate = (float64(totalPassedSkills) / float64(totalAssessedSkills)) * 100
	}

	result := &models.TotalPassedSkillsResult{
		UserID:              userID,
		TotalPassedSkills:   totalPassedSkills,
		TotalAssessedSkills: totalAssessedSkills,
		PassRate:            passRate,
		CompletionThreshold: completionThreshold,
		CalculationMethod:   "aggregated_assessment",
		LastCalculated:      time.Now(),
	}

	return result, nil
}
