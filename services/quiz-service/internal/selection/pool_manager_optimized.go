// services/quiz-service/internal/selection/pool_manager_optimized.go
package selection

import (
	"context"
	"fmt"
	"quiz-service/internal/models"
	"time"
)

// GetGlobalPoolWithBloomCached retrieves global question pool with skill-based caching
// This optimized version uses skill-based cache instead of calling database repeatedly
func (pm *PoolManager) GetGlobalPoolWithBloomCached(ctx context.Context, skillInfo *SkillInfo) (*QuizPool, error) {
	// Validate inputs to prevent nil pointer panics
	if skillInfo == nil {
		return nil, fmt.Errorf("skillInfo cannot be nil")
	}
	if pm == nil {
		return nil, fmt.Errorf("pool manager is nil")
	}
	if pm.questionRepo == nil {
		return nil, fmt.Errorf("question repository is not initialized")
	}

	fmt.Printf("[PoolManager] DEBUG: Starting GetGlobalPoolWithBloomCached for skill: %s (ID: %s)\n", skillInfo.Name, skillInfo.ID)

	// Check skill-based cache first
	skillCache := GetSkillCache()
	if skillCache == nil {
		return nil, fmt.Errorf("skill cache is not initialized")
	}

	startTime := time.Now()
	cachedQuestions, cacheHit := skillCache.GetCachedQuestions(skillInfo.Name, "session_creation")
	cacheCheckDuration := time.Since(startTime)

	var questions []models.Question
	var err error
	var dbFetchDuration time.Duration

	if cacheHit {
		// Cache hit - use cached questions
		questions = cachedQuestions
		fmt.Printf("[PoolManager] DEBUG: Cache HIT for skill '%s' - %d questions retrieved from cache in %v\n",
			skillInfo.Name, len(questions), cacheCheckDuration)
	} else {
		// Cache miss - fetch from database and cache
		fmt.Printf("[PoolManager] DEBUG: Cache MISS for skill '%s' - fetching from database (cache check took %v)\n",
			skillInfo.Name, cacheCheckDuration)

		dbStartTime := time.Now()
		questions, err = pm.questionRepo.FindAll(ctx)
		dbFetchDuration = time.Since(dbStartTime)

		if err != nil {
			fmt.Printf("[PoolManager] ERROR: Database fetch failed for skill '%s': %v\n", skillInfo.Name, err)
			return nil, fmt.Errorf("failed to get global questions: %w", err)
		}

		fmt.Printf("[PoolManager] DEBUG: Database fetch COMPLETED for skill '%s' - retrieved %d questions in %v\n",
			skillInfo.Name, len(questions), dbFetchDuration)

		// Cache the questions for future use
		cacheStartTime := time.Now()
		skillCache.SetCachedQuestions(skillInfo.Name, "session_creation", questions)
		cacheDuration := time.Since(cacheStartTime)

		fmt.Printf("[PoolManager] DEBUG: Questions CACHED for skill '%s' - cached %d questions in %v\n",
			skillInfo.Name, len(questions), cacheDuration)
	}

	fmt.Printf("[PoolManager] DEBUG: Total questions retrieved: %d\n", len(questions))

	// Filter questions by skill tags (same as original logic)
	filterStartTime := time.Now()
	var filteredQuestions []models.Question
	for _, q := range questions {
		if pm.matchesSkillTags(q.TopicTags, skillInfo.Tags) {
			filteredQuestions = append(filteredQuestions, q)
		}
	}
	filterDuration := time.Since(filterStartTime)

	fmt.Printf("[PoolManager] DEBUG: Question filtering COMPLETED - %d questions match skill tags (took %v)\n",
		len(filteredQuestions), filterDuration)

	// Calculate Bloom's distribution (same as original logic)
	distributionStartTime := time.Now()
	bloomDistribution := make(map[string]int)
	difficultyMatrix := make(map[string]map[string]int)

	// Initialize difficulty matrix
	difficulties := []string{"easy", "medium", "hard"}
	for _, difficulty := range difficulties {
		difficultyMatrix[difficulty] = make(map[string]int)
	}

	for _, question := range filteredQuestions {
		// NOTE: Score initialization removed - using pure service-based scoring
		// Question scoring is now handled directly through BloomScoringService

		// Count by Bloom level
		if question.BloomLevel != "" {
			bloomDistribution[question.BloomLevel]++
		}

		// Count by difficulty and Bloom level
		if difficultyMatrix[question.DifficultyLevel] != nil {
			if question.BloomLevel != "" {
				difficultyMatrix[question.DifficultyLevel][question.BloomLevel]++
			}
		}
	}
	distributionDuration := time.Since(distributionStartTime)

	fmt.Printf("[PoolManager] DEBUG: Bloom distribution calculation COMPLETED in %v\n", distributionDuration)
	fmt.Printf("[PoolManager] DEBUG: Bloom distribution: %+v\n", bloomDistribution)

	pool := &QuizPool{
		ID:                fmt.Sprintf("global_skill_%s", skillInfo.ID),
		Name:              fmt.Sprintf("Global Pool for %s", skillInfo.Name),
		Description:       fmt.Sprintf("Global question pool for skill: %s", skillInfo.Name),
		SkillID:           skillInfo.ID,
		SkillTags:         skillInfo.Tags,
		Questions:         filteredQuestions,
		TotalCount:        len(filteredQuestions),
		BloomDistribution: bloomDistribution,
		DifficultyMatrix:  difficultyMatrix,
	}

	totalDuration := time.Since(startTime)
	fmt.Printf("[PoolManager] DEBUG: GetGlobalPoolWithBloomCached COMPLETED for skill '%s' - total time: %v, cache hit: %v\n",
		skillInfo.Name, totalDuration, cacheHit)

	return pool, nil
}

// BatchGeneratePoolsFromCache generates multiple pools from cached skill questions
// This eliminates the need for multiple database calls during session creation
func (pm *PoolManager) BatchGeneratePoolsFromCache(
	ctx context.Context,
	skillInfo *SkillInfo,
	sessionID string,
	poolConfigs []PoolConfig,
) error {
	// Validate inputs to prevent nil pointer panics
	if pm == nil {
		return fmt.Errorf("pool manager is nil")
	}
	if skillInfo == nil {
		return fmt.Errorf("skillInfo cannot be nil")
	}
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	if len(poolConfigs) == 0 {
		return fmt.Errorf("poolConfigs cannot be empty")
	}
	if pm.questionRepo == nil {
		return fmt.Errorf("question repository is not initialized")
	}

	fmt.Printf("[BatchPool] DEBUG: Starting batch pool generation for session %s with skill '%s'\n", sessionID, skillInfo.Name)
	fmt.Printf("[BatchPool] DEBUG: Number of pool configurations: %d\n", len(poolConfigs))

	batchStartTime := time.Now()

	// Get cached questions once
	skillCache := GetSkillCache()
	if skillCache == nil {
		return fmt.Errorf("skill cache is not initialized")
	}

	cacheStartTime := time.Now()
	cachedQuestions, cacheHit := skillCache.GetCachedQuestions(skillInfo.Name, "session_creation")
	cacheCheckDuration := time.Since(cacheStartTime)

	var allQuestions []models.Question
	var err error
	var dbFetchDuration time.Duration

	if cacheHit {
		allQuestions = cachedQuestions
		fmt.Printf("[BatchPool] DEBUG: Using CACHED questions for skill '%s' - %d questions (cache check: %v)\n",
			skillInfo.Name, len(allQuestions), cacheCheckDuration)
	} else {
		// Fallback to database if cache miss
		fmt.Printf("[BatchPool] DEBUG: Cache MISS - fetching from database for skill '%s'\n", skillInfo.Name)
		dbStartTime := time.Now()
		allQuestions, err = pm.questionRepo.FindAll(ctx)
		dbFetchDuration = time.Since(dbStartTime)

		if err != nil {
			fmt.Printf("[BatchPool] ERROR: Database fetch failed: %v\n", err)
			return fmt.Errorf("failed to get global questions: %w", err)
		}

		fmt.Printf("[BatchPool] DEBUG: Database fetch COMPLETED - %d questions in %v\n", len(allQuestions), dbFetchDuration)

		// Cache for future use
		cacheStoreTime := time.Now()
		skillCache.SetCachedQuestions(skillInfo.Name, "session_creation", allQuestions)
		cacheStoreDuration := time.Since(cacheStoreTime)
		fmt.Printf("[BatchPool] DEBUG: Questions CACHED in %v\n", cacheStoreDuration)
	}

	// Filter questions by skill tags once
	filterStartTime := time.Now()
	var filteredQuestions []models.Question
	for _, q := range allQuestions {
		if pm.matchesSkillTags(q.TopicTags, skillInfo.Tags) {
			filteredQuestions = append(filteredQuestions, q)
		}
	}
	filterDuration := time.Since(filterStartTime)
	fmt.Printf("[BatchPool] DEBUG: Question filtering COMPLETED - %d/%d questions match skill tags (took %v)\n",
		len(filteredQuestions), len(allQuestions), filterDuration)

	// Generate all pools from the same filtered question set
	poolGenerationStartTime := time.Now()
	for i, config := range poolConfigs {
		poolStartTime := time.Now()
		cacheKey := fmt.Sprintf("session_%s_%s_%s", sessionID, config.Stage, config.Type)

		fmt.Printf("[BatchPool] DEBUG: Generating pool %d/%d - %s (stage: %s, type: %s, size: %d)\n",
			i+1, len(poolConfigs), cacheKey, config.Stage, config.Type, config.PoolSize)

		// Select questions for this specific pool configuration
		result, err := pm.selectQuestionsFromFiltered(
			filteredQuestions,
			skillInfo,
			config.Stage,
			config.PoolSize,
			[]string{}, // No exclusions for pool generation
			config.BloomDistribution,
		)
		if err != nil {
			fmt.Printf("[BatchPool] ERROR: Failed to generate pool %s: %v\n", cacheKey, err)
			return fmt.Errorf("failed to generate pool %s: %w", cacheKey, err)
		}

		// Create and cache the pool
		pool := &QuizPool{
			ID:                cacheKey,
			Name:              fmt.Sprintf("Pool %s", cacheKey),
			Description:       fmt.Sprintf("Generated pool for %s %s", config.Stage, config.Type),
			SkillID:           skillInfo.ID,
			SkillTags:         skillInfo.Tags,
			Questions:         result.Questions,
			TotalCount:        len(result.Questions),
			BloomDistribution: result.BloomCoverage,
			DifficultyMatrix:  make(map[string]map[string]int), // Could be calculated if needed
		}

		// Cache the generated pool
		SetPoolInCache(cacheKey, pool)
		poolDuration := time.Since(poolStartTime)

		fmt.Printf("[BatchPool] DEBUG: Pool %s COMPLETED - %d questions generated in %v\n",
			cacheKey, pool.TotalCount, poolDuration)
		fmt.Printf("Generated and cached pool %s with %d questions\n", cacheKey, pool.TotalCount)
	}

	poolGenerationDuration := time.Since(poolGenerationStartTime)
	totalDuration := time.Since(batchStartTime)

	fmt.Printf("[BatchPool] DEBUG: Batch pool generation COMPLETED for session %s\n", sessionID)
	fmt.Printf("[BatchPool] DEBUG: Generated %d pools in %v (total operation: %v, cache hit: %v)\n",
		len(poolConfigs), poolGenerationDuration, totalDuration, cacheHit)

	return nil
}

// PoolConfig represents configuration for pool generation
type PoolConfig struct {
	Stage             string             `json:"stage"`
	Type              string             `json:"type"` // "initial" or "recovery"
	PoolSize          int                `json:"pool_size"`
	BloomDistribution map[string]float64 `json:"bloom_distribution"`
}

// selectQuestionsFromFiltered selects questions from pre-filtered set
// This avoids re-filtering the same questions multiple times
func (pm *PoolManager) selectQuestionsFromFiltered(
	filteredQuestions []models.Question,
	skillInfo *SkillInfo,
	difficulty string,
	count int,
	excludeIDs []string,
	bloomDistribution map[string]float64,
) (*SelectionResult, error) {
	// Validate inputs to prevent nil pointer panics
	if pm == nil {
		return nil, fmt.Errorf("pool manager is nil")
	}
	if skillInfo == nil {
		return nil, fmt.Errorf("skillInfo cannot be nil")
	}
	if filteredQuestions == nil {
		return nil, fmt.Errorf("filteredQuestions cannot be nil")
	}

	// Create temporary pool from filtered questions
	tempPool := &QuizPool{
		Questions:  filteredQuestions,
		TotalCount: len(filteredQuestions),
	}

	// Use existing selector logic
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

	// Validate selector exists to prevent nil pointer panic
	if pm.selector == nil {
		return nil, fmt.Errorf("selector is not initialized")
	}

	return pm.selector.SelectQuestionsWithBloom(tempPool.Questions, criteria, bloomDistribution)
}
