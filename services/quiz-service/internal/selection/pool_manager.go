package selection

import (
	"context"
	"fmt"
	"quiz-service/internal/repository"
)

// PoolManager manages quiz pools and question selection
type PoolManager struct {
	questionRepo *repository.QuestionRepository
	selector     *WeightedSelector
}

// NewPoolManager creates a new pool manager
func NewPoolManager(questionRepo *repository.QuestionRepository) *PoolManager {
	return &PoolManager{
		questionRepo: questionRepo,
		selector:     NewWeightedSelector(),
	}
}

// GetQuizPool retrieves all questions for a quiz with skill information
func (pm *PoolManager) GetQuizPool(ctx context.Context, quizID string, skillInfo *SkillInfo) (*QuizPool, error) {
	// Get all questions for the quiz
	questions, err := pm.questionRepo.FindByQuizID(ctx, quizID)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	return &QuizPool{
		ID:         quizID,
		SkillID:    skillInfo.ID,
		SkillTags:  skillInfo.Tags,
		Questions:  questions,
		TotalCount: len(questions),
	}, nil
}

// SelectAdaptiveQuestions selects questions for adaptive quiz stage
func (pm *PoolManager) SelectAdaptiveQuestions(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
) (*SelectionResult, error) {
	// Get quiz pool
	pool, err := pm.GetQuizPool(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Prepare selection criteria
	criteria := &SelectionCriteria{
		SkillID:        skillInfo.ID,
		SkillTags:      skillInfo.Tags,
		Difficulty:     difficulty,
		ExcludeIDs:     excludeIDs,
		Count:          count,
		MinTagMatch:    0,   // Accept any, but prefer higher matches
		WeightExponent: 2.0, // Square the match count for stronger preference
	}

	// Select questions using weighted selection
	result, err := pm.selector.SelectQuestions(pool.Questions, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to select questions: %w", err)
	}

	// If we don't have enough questions with the criteria, relax constraints
	if len(result.Questions) < count {
		// Try without difficulty filter
		criteria.Difficulty = ""
		result, err = pm.selector.SelectQuestions(pool.Questions, criteria)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// SelectRecoveryQuestions selects questions for recovery stage
func (pm *PoolManager) SelectRecoveryQuestions(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
) (*SelectionResult, error) {
	// For recovery, we might want to select questions with higher tag matches
	// to give the user a better chance
	pool, err := pm.GetQuizPool(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	criteria := &SelectionCriteria{
		SkillID:        skillInfo.ID,
		SkillTags:      skillInfo.Tags,
		Difficulty:     difficulty,
		ExcludeIDs:     excludeIDs,
		Count:          count,
		MinTagMatch:    1,   // Prefer questions with at least 1 tag match for recovery
		WeightExponent: 1.5, // Less aggressive weighting for recovery
	}

	result, err := pm.selector.SelectQuestions(pool.Questions, criteria)
	if err != nil {
		return nil, err
	}

	// If not enough questions, relax the minimum match requirement
	if len(result.Questions) < count {
		criteria.MinTagMatch = 0
		result, err = pm.selector.SelectQuestions(pool.Questions, criteria)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// GetQuestionDistribution analyzes question distribution in a pool
func (pm *PoolManager) GetQuestionDistribution(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
) (map[string]interface{}, error) {
	pool, err := pm.GetQuizPool(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Count by difficulty
	difficultyCount := map[string]int{
		"easy":   0,
		"medium": 0,
		"hard":   0,
	}

	// Count by tag matches
	tagMatchDistribution := map[int]int{}

	for _, q := range pool.Questions {
		// Count difficulty
		difficultyCount[q.DifficultyLevel]++

		// Count tag matches
		matches, _ := pm.selector.countTagMatches(q.TopicTags, skillInfo.Tags)
		tagMatchDistribution[matches]++
	}

	// Get top matching questions for analysis
	topMatches := pm.selector.GetTopMatchingQuestions(pool.Questions, skillInfo.Tags, 10)

	return map[string]interface{}{
		"total_questions":         pool.TotalCount,
		"difficulty_distribution": difficultyCount,
		"tag_match_distribution":  tagMatchDistribution,
		"top_matching_questions":  topMatches,
		"skill_tags":              skillInfo.Tags,
	}, nil
}

// ValidateQuizPool checks if a quiz pool has enough questions for adaptive quiz
func (pm *PoolManager) ValidateQuizPool(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
) (bool, map[string]int, error) {
	pool, err := pm.GetQuizPool(ctx, quizID, skillInfo)
	if err != nil {
		return false, nil, err
	}

	// Count questions by difficulty
	counts := map[string]int{
		"easy":   0,
		"medium": 0,
		"hard":   0,
	}

	for _, q := range pool.Questions {
		counts[q.DifficultyLevel]++
	}

	// Minimum requirements for adaptive quiz
	// Easy: 5 initial + 3 recovery = 8
	// Medium: 5 initial + 3 recovery = 8
	// Hard: 5 initial + 3 recovery = 8
	minRequired := map[string]int{
		"easy":   8,
		"medium": 8,
		"hard":   8,
	}

	isValid := true
	for difficulty, required := range minRequired {
		if counts[difficulty] < required {
			isValid = false
			break
		}
	}

	return isValid, counts, nil
}
