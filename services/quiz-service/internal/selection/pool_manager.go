package selection

import (
	"context"
	"fmt"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
	"strings"
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

// SelectAdaptiveQuestionsWithBloom selects questions with Bloom's level consideration
func (pm *PoolManager) SelectAdaptiveQuestionsWithBloom(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// Get quiz pool
	pool, err := pm.GetQuizPoolWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Prepare selection criteria with Bloom's distribution
	criteria := &SelectionCriteria{
		SkillID:           skillInfo.ID,
		SkillTags:         skillInfo.Tags,
		Difficulty:        difficulty,
		ExcludeIDs:        excludeIDs,
		Count:             count,
		MinTagMatch:       0,
		WeightExponent:    2.0,
		BloomDistribution: bloomDistribution,
	}

	// Select questions using enhanced weighted selection
	result, err := pm.selector.SelectQuestionsWithBloom(pool.Questions, criteria, bloomDistribution)
	if err != nil {
		return nil, fmt.Errorf("failed to select questions: %w", err)
	}

	// Enhance result with coverage statistics
	pm.enhanceResultStats(result, pool)

	// If we don't have enough questions, try relaxing constraints
	if len(result.Questions) < count {
		// Try with relaxed Bloom's distribution
		relaxedDist := pm.getRelaxedBloomDistribution(difficulty)
		criteria.BloomDistribution = relaxedDist
		result, err = pm.selector.SelectQuestionsWithBloom(pool.Questions, criteria, relaxedDist)
		if err != nil {
			return nil, err
		}
		pm.enhanceResultStats(result, pool)
	}

	return result, nil
}

// SelectRecoveryQuestionsWithBloom selects recovery questions with Bloom's consideration
func (pm *PoolManager) SelectRecoveryQuestionsWithBloom(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// For recovery, use a simplified Bloom's distribution
	recoveryDist := pm.getRecoveryBloomDistribution(difficulty)

	pool, err := pm.GetQuizPoolWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	criteria := &SelectionCriteria{
		SkillID:           skillInfo.ID,
		SkillTags:         skillInfo.Tags,
		Difficulty:        difficulty,
		ExcludeIDs:        excludeIDs,
		Count:             count,
		MinTagMatch:       1,   // Prefer at least 1 tag match for recovery
		WeightExponent:    1.5, // Less aggressive weighting
		BloomDistribution: recoveryDist,
	}

	result, err := pm.selector.SelectQuestionsWithBloom(pool.Questions, criteria, recoveryDist)
	if err != nil {
		return nil, err
	}

	pm.enhanceResultStats(result, pool)

	// If not enough questions, relax constraints
	if len(result.Questions) < count {
		criteria.MinTagMatch = 0
		result, err = pm.selector.SelectQuestionsWithBloom(pool.Questions, criteria, recoveryDist)
		if err != nil {
			return nil, err
		}
		pm.enhanceResultStats(result, pool)
	}

	return result, nil
}

// GetQuizPoolWithBloom retrieves quiz pool with Bloom's level analysis
func (pm *PoolManager) GetQuizPoolWithBloom(ctx context.Context, quizID string, skillInfo *SkillInfo) (*QuizPool, error) {
	// Get all questions for the quiz
	questions, err := pm.questionRepo.FindByQuizID(ctx, quizID)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	// Calculate Bloom's distribution
	bloomDist := make(map[string]int)
	diffMatrix := make(map[string]map[string]int)

	for _, q := range questions {
		// Count Bloom's levels
		bloomLevel := q.BloomLevel
		if bloomLevel == "" {
			bloomLevel = "unknown"
		}
		bloomDist[bloomLevel]++

		// Build difficulty-Bloom matrix
		if diffMatrix[q.DifficultyLevel] == nil {
			diffMatrix[q.DifficultyLevel] = make(map[string]int)
		}
		diffMatrix[q.DifficultyLevel][bloomLevel]++
	}

	return &QuizPool{
		ID:                quizID,
		SkillID:           skillInfo.ID,
		SkillTags:         skillInfo.Tags,
		Questions:         questions,
		TotalCount:        len(questions),
		BloomDistribution: bloomDist,
		DifficultyMatrix:  diffMatrix,
	}, nil
}

// ValidateQuizPoolWithBloom validates if pool has sufficient questions across Bloom's levels
func (pm *PoolManager) ValidateQuizPoolWithBloom(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
) (bool, *QuizPoolValidation, error) {
	pool, err := pm.GetQuizPoolWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return false, nil, err
	}

	validation := &QuizPoolValidation{
		TotalQuestions:        pool.TotalCount,
		DifficultyCount:       make(map[string]int),
		BloomCount:            pool.BloomDistribution,
		DifficultyBloomMatrix: pool.DifficultyMatrix,
		MissingLevels:         []string{},
		Warnings:              []string{},
	}

	// Count by difficulty
	for _, q := range pool.Questions {
		validation.DifficultyCount[q.DifficultyLevel]++
	}

	// Check minimum requirements for adaptive quiz
	requiredPerDifficulty := map[string]int{
		"easy":   8, // 5 initial + 3 recovery
		"medium": 8,
		"hard":   8,
	}

	isValid := true
	for difficulty, required := range requiredPerDifficulty {
		actual := validation.DifficultyCount[difficulty]
		if actual < required {
			isValid = false
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("Insufficient %s questions: need %d, have %d", difficulty, required, actual))
		}
	}

	// Check Bloom's level coverage
	requiredBloomLevels := []string{"remember", "understand", "apply", "analyze"}
	for _, level := range requiredBloomLevels {
		if validation.BloomCount[level] == 0 {
			validation.MissingLevels = append(validation.MissingLevels, level)
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("No questions for Bloom's level: %s", level))
		}
	}

	// Check distribution balance
	pm.validateBloomBalance(validation)

	validation.IsValid = isValid && len(validation.MissingLevels) == 0

	return validation.IsValid, validation, nil
}

// validateBloomBalance checks if Bloom's distribution is balanced
func (pm *PoolManager) validateBloomBalance(validation *QuizPoolValidation) {
	// Check if any difficulty level lacks diversity in Bloom's levels
	for difficulty, bloomCounts := range validation.DifficultyBloomMatrix {
		uniqueLevels := 0
		for _, count := range bloomCounts {
			if count > 0 {
				uniqueLevels++
			}
		}

		if uniqueLevels < 2 {
			validation.Warnings = append(validation.Warnings,
				fmt.Sprintf("%s difficulty has insufficient Bloom's diversity (only %d levels)",
					difficulty, uniqueLevels))
		}
	}
}

// getRecoveryBloomDistribution returns simplified Bloom's distribution for recovery
func (pm *PoolManager) getRecoveryBloomDistribution(difficulty string) map[string]float64 {
	// For recovery, focus on lower Bloom's levels to help students succeed
	switch difficulty {
	case "easy":
		return map[string]float64{
			"remember":   0.6,
			"understand": 0.4,
		}
	case "medium":
		return map[string]float64{
			"remember":   0.3,
			"understand": 0.4,
			"apply":      0.3,
		}
	case "hard":
		return map[string]float64{
			"understand": 0.3,
			"apply":      0.4,
			"analyze":    0.3,
		}
	default:
		return map[string]float64{
			"remember":   0.4,
			"understand": 0.4,
			"apply":      0.2,
		}
	}
}

// getRelaxedBloomDistribution returns a more flexible Bloom's distribution
func (pm *PoolManager) getRelaxedBloomDistribution(difficulty string) map[string]float64 {
	// More balanced distribution when strict requirements can't be met
	return map[string]float64{
		"remember":   0.2,
		"understand": 0.2,
		"apply":      0.2,
		"analyze":    0.2,
		"evaluate":   0.1,
		"create":     0.1,
	}
}

// enhanceResultStats adds detailed statistics to selection result
func (pm *PoolManager) enhanceResultStats(result *SelectionResult, pool *QuizPool) {
	// Calculate Bloom's coverage
	bloomCoverage := make(map[string]int)
	tagCoverage := make(map[string]int)

	for _, q := range result.Questions {
		bloomCoverage[q.BloomLevel]++
		for _, tag := range q.TopicTags {
			tagCoverage[tag]++
		}
	}

	result.BloomCoverage = bloomCoverage
	result.TagCoverage = tagCoverage

	// Add selection statistics
	stats := SelectionStats{
		TotalQuestionsScanned: pool.TotalCount,
		QuestionsFiltered:     pool.TotalCount - result.TotalCandidates,
		BloomLevelHits:        bloomCoverage,
		DifficultyHits:        make(map[string]int),
	}

	for _, q := range result.Questions {
		stats.DifficultyHits[q.DifficultyLevel]++
	}

	// Calculate averages
	if len(result.Weights) > 0 {
		totalWeight := 0.0
		totalMatches := 0
		for _, w := range result.Weights {
			totalWeight += w.Weight
			totalMatches += w.TagMatches
		}
		stats.AverageWeight = totalWeight / float64(len(result.Weights))
		stats.AverageTagMatch = float64(totalMatches) / float64(len(result.Weights))
	}

	result.SelectionStats = stats
}

// GetQuestionDistributionWithBloom analyzes question distribution including Bloom's levels
func (pm *PoolManager) GetQuestionDistributionWithBloom(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
) (map[string]interface{}, error) {
	pool, err := pm.GetQuizPoolWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Analyze tag matching distribution
	tagMatchDistribution := map[int]int{}
	bloomTagCorrelation := make(map[string]map[int]int) // bloom_level -> tag_matches -> count

	for _, q := range pool.Questions {
		matches, _ := pm.selector.countTagMatches(q.TopicTags, skillInfo.Tags)
		tagMatchDistribution[matches]++

		// Track correlation between Bloom's level and tag matches
		bloomLevel := q.BloomLevel
		if bloomLevel == "" {
			bloomLevel = "unknown"
		}
		if bloomTagCorrelation[bloomLevel] == nil {
			bloomTagCorrelation[bloomLevel] = make(map[int]int)
		}
		bloomTagCorrelation[bloomLevel][matches]++
	}

	// Get top matching questions for each Bloom's level
	topByBloom := make(map[string][]WeightedQuestion)
	for level := range BloomLevelWeights {
		topByBloom[level] = pm.getTopMatchingByBloom(pool.Questions, skillInfo.Tags, level, 3)
	}

	return map[string]interface{}{
		"total_questions":         pool.TotalCount,
		"difficulty_distribution": pool.DifficultyMatrix,
		"bloom_distribution":      pool.BloomDistribution,
		"tag_match_distribution":  tagMatchDistribution,
		"bloom_tag_correlation":   bloomTagCorrelation,
		"top_questions_by_bloom":  topByBloom,
		"skill_tags":              skillInfo.Tags,
	}, nil
}

// getTopMatchingByBloom returns top matching questions for a specific Bloom's level
func (pm *PoolManager) getTopMatchingByBloom(
	questions []models.Question,
	skillTags []string,
	bloomLevel string,
	limit int,
) []WeightedQuestion {
	var filtered []models.Question
	for _, q := range questions {
		if q.BloomLevel == bloomLevel {
			filtered = append(filtered, q)
		}
	}

	return pm.selector.GetTopMatchingQuestions(filtered, skillTags, limit)
}

// Legacy methods preserved for backward compatibility
func (pm *PoolManager) GetQuizPool(ctx context.Context, quizID string, skillInfo *SkillInfo) (*QuizPool, error) {
	return pm.GetQuizPoolWithBloom(ctx, quizID, skillInfo)
}

func (pm *PoolManager) SelectAdaptiveQuestions(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
) (*SelectionResult, error) {
	// Use default Bloom's distribution
	bloomDist := DifficultyBloomMatrix[difficulty]
	if bloomDist == nil {
		bloomDist = map[string]float64{
			"remember": 0.2, "understand": 0.2, "apply": 0.2,
			"analyze": 0.2, "evaluate": 0.1, "create": 0.1,
		}
	}
	return pm.SelectAdaptiveQuestionsWithBloom(ctx, quizID, skillInfo, difficulty, count, excludeIDs, bloomDist)
}

func (pm *PoolManager) SelectRecoveryQuestions(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
) (*SelectionResult, error) {
	bloomDist := pm.getRecoveryBloomDistribution(difficulty)
	return pm.SelectRecoveryQuestionsWithBloom(ctx, quizID, skillInfo, difficulty, count, excludeIDs, bloomDist)
}

func (pm *PoolManager) ValidateQuizPool(
	ctx context.Context,
	quizID string,
	skillInfo *SkillInfo,
) (bool, map[string]int, error) {
	isValid, validation, err := pm.ValidateQuizPoolWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return false, nil, err
	}
	return isValid, validation.DifficultyCount, nil
}

func (pm *PoolManager) SelectAdaptiveQuestionsWithEnhancedWeights(
	ctx context.Context,
	quizID string,
	skillInfo *EnhancedSkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// Get quiz pool
	pool, err := pm.GetQuizPoolForEnhancedSkill(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Prepare enhanced selection criteria
	criteria := &EnhancedSelectionCriteria{
		SkillInfo:         skillInfo,
		Difficulty:        difficulty,
		ExcludeIDs:        excludeIDs,
		Count:             count,
		MinPrimaryMatch:   0, // Don't require but prefer
		MinSecondaryMatch: 0,
		PreferExactSkill:  true,
		WeightExponent:    2.0,
		BloomDistribution: bloomDistribution,
	}

	// Use enhanced weighted selection
	result, err := pm.selector.SelectQuestionsWithEnhancedWeights(
		pool.Questions,
		criteria,
		skillInfo.TagWeights,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select questions with enhanced weights: %w", err)
	}

	// Enhance result with coverage statistics
	pm.enhanceResultStats(result, pool)

	// If we don't have enough questions, try relaxing constraints
	if len(result.Questions) < count {
		// Relax to allow any tag match
		criteria.MinPrimaryMatch = 0
		criteria.MinSecondaryMatch = 0
		criteria.WeightExponent = 1.5 // Less aggressive weighting

		result, err = pm.selector.SelectQuestionsWithEnhancedWeights(
			pool.Questions,
			criteria,
			skillInfo.TagWeights,
		)
		if err != nil {
			return nil, err
		}
		pm.enhanceResultStats(result, pool)
	}

	return result, nil
}

// SelectRecoveryQuestionsWithEnhancedWeights selects recovery questions with tag weights
func (pm *PoolManager) SelectRecoveryQuestionsWithEnhancedWeights(
	ctx context.Context,
	quizID string,
	skillInfo *EnhancedSkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// For recovery, use simplified Bloom's distribution
	recoveryDist := pm.getRecoveryBloomDistribution(difficulty)

	pool, err := pm.GetQuizPoolForEnhancedSkill(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Adjust weights for recovery - emphasize primary tags more
	recoveryWeights := TagWeightConfig{
		PrimaryWeight:   skillInfo.TagWeights.PrimaryWeight * 1.5, // Boost primary tags
		SecondaryWeight: skillInfo.TagWeights.SecondaryWeight,
		RelatedWeight:   skillInfo.TagWeights.RelatedWeight * 0.5, // Reduce related tags
		ExactMatchBonus: skillInfo.TagWeights.ExactMatchBonus,
	}

	enhancedSkillInfo := *skillInfo
	enhancedSkillInfo.TagWeights = recoveryWeights

	criteria := &EnhancedSelectionCriteria{
		SkillInfo:         &enhancedSkillInfo,
		Difficulty:        difficulty,
		ExcludeIDs:        excludeIDs,
		Count:             count,
		MinPrimaryMatch:   0, // Prefer but don't require
		MinSecondaryMatch: 0,
		PreferExactSkill:  true,
		WeightExponent:    1.5, // Less aggressive for recovery
		BloomDistribution: recoveryDist,
	}

	result, err := pm.selector.SelectQuestionsWithEnhancedWeights(
		pool.Questions,
		criteria,
		recoveryWeights,
	)
	if err != nil {
		return nil, err
	}

	pm.enhanceResultStats(result, pool)

	return result, nil
}

// GetQuizPoolForEnhancedSkill retrieves quiz pool for enhanced skill info
func (pm *PoolManager) GetQuizPoolForEnhancedSkill(
	ctx context.Context,
	quizID string,
	skillInfo *EnhancedSkillInfo,
) (*QuizPool, error) {
	// Get all questions for the quiz
	questions, err := pm.questionRepo.FindByQuizID(ctx, quizID)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	// Merge all tags for pool creation
	allTags := make([]string, 0)
	allTags = append(allTags, skillInfo.PrimaryTags...)
	allTags = append(allTags, skillInfo.SecondaryTags...)
	allTags = append(allTags, skillInfo.RelatedTags...)

	// Calculate Bloom's distribution and difficulty matrix
	bloomDist := make(map[string]int)
	diffMatrix := make(map[string]map[string]int)

	// Also track tag category distribution
	tagCategoryMatrix := make(map[string]map[string]int) // difficulty -> tag_category -> count

	for _, q := range questions {
		// Count Bloom's levels
		bloomLevel := q.BloomLevel
		if bloomLevel == "" {
			bloomLevel = "unknown"
		}
		bloomDist[bloomLevel]++

		// Build difficulty-Bloom matrix
		if diffMatrix[q.DifficultyLevel] == nil {
			diffMatrix[q.DifficultyLevel] = make(map[string]int)
		}
		diffMatrix[q.DifficultyLevel][bloomLevel]++

		// Track tag category matches
		if tagCategoryMatrix[q.DifficultyLevel] == nil {
			tagCategoryMatrix[q.DifficultyLevel] = make(map[string]int)
		}

		primaryMatches := pm.countTagMatches(q.TopicTags, skillInfo.PrimaryTags)
		secondaryMatches := pm.countTagMatches(q.TopicTags, skillInfo.SecondaryTags)
		relatedMatches := pm.countTagMatches(q.TopicTags, skillInfo.RelatedTags)

		if primaryMatches > 0 {
			tagCategoryMatrix[q.DifficultyLevel]["primary"]++
		}
		if secondaryMatches > 0 {
			tagCategoryMatrix[q.DifficultyLevel]["secondary"]++
		}
		if relatedMatches > 0 {
			tagCategoryMatrix[q.DifficultyLevel]["related"]++
		}
	}

	return &QuizPool{
		ID:                quizID,
		SkillID:           skillInfo.ID,
		SkillTags:         allTags,
		Questions:         questions,
		TotalCount:        len(questions),
		BloomDistribution: bloomDist,
		DifficultyMatrix:  diffMatrix,
	}, nil
}

// Add helper method to count tag matches
func (pm *PoolManager) countTagMatches(questionTags []string, targetTags []string) int {
	matches := 0
	targetMap := make(map[string]bool)
	for _, tag := range targetTags {
		targetMap[strings.ToLower(tag)] = true
	}

	for _, qTag := range questionTags {
		if targetMap[strings.ToLower(qTag)] {
			matches++
		}
	}

	return matches
}
