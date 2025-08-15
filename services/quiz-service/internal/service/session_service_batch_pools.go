// services/quiz-service/internal/service/session_service_batch_pools.go
package service

import (
	"context"
	"fmt"
	"log"
	"quiz-service/internal/models"
	"quiz-service/internal/selection"
	"time"
)

// generateSessionPoolOptimized creates and caches pools for a session using skill-based caching
// This replaces the original generateSessionPool method to eliminate redundant database calls
func (s *SessionService) generateSessionPoolOptimized(ctx context.Context, sessionID string, session *models.QuizSession) error {
	// Validate inputs to prevent nil pointer panics
	if s == nil {
		return fmt.Errorf("session service is nil")
	}
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	if s.poolManager == nil {
		return fmt.Errorf("pool manager is not initialized")
	}

	log.Printf("[POOL_GENERATION] Starting optimized pool generation for session %s", sessionID)

	sessionStartTime := time.Now()
	skillInfo := s.getSkillInfoFromSession(session)
	if skillInfo == nil {
		log.Printf("[POOL_GENERATION] ERROR: Skill information not found for session %s", sessionID)
		return fmt.Errorf("skill information not found for session")
	}

	log.Printf("[POOL_GENERATION] Retrieved skill info for session %s - skill: '%s' (ID: %s), tags: %v",
		sessionID, skillInfo.Name, skillInfo.ID, skillInfo.Tags)

	// Define all pool configurations that need to be generated
	poolConfigs := []selection.PoolConfig{
		// Easy stage pools
		{
			Stage:             "easy",
			Type:              "initial",
			PoolSize:          50,
			BloomDistribution: s.getBloomDistribution("easy"),
		},
		{
			Stage:             "easy",
			Type:              "recovery",
			PoolSize:          25,
			BloomDistribution: s.getRecoveryBloomDistribution("easy"),
		},
		// Medium stage pools
		{
			Stage:             "medium",
			Type:              "initial",
			PoolSize:          50,
			BloomDistribution: s.getBloomDistribution("medium"),
		},
		{
			Stage:             "medium",
			Type:              "recovery",
			PoolSize:          25,
			BloomDistribution: s.getRecoveryBloomDistribution("medium"),
		},
		// Hard stage pools
		{
			Stage:             "hard",
			Type:              "initial",
			PoolSize:          50,
			BloomDistribution: s.getBloomDistribution("hard"),
		},
		{
			Stage:             "hard",
			Type:              "recovery",
			PoolSize:          25,
			BloomDistribution: s.getRecoveryBloomDistribution("hard"),
		},
	}

	log.Printf("[POOL_GENERATION] Created %d pool configurations for session %s", len(poolConfigs), sessionID)
	log.Printf("[POOL_GENERATION] Pool breakdown - Easy: 2 (initial+recovery), Medium: 2, Hard: 2")

	// Generate all pools in a single batch operation using skill cache
	batchStartTime := time.Now()
	err := s.poolManager.BatchGeneratePoolsFromCache(ctx, skillInfo, sessionID, poolConfigs)
	batchDuration := time.Since(batchStartTime)

	if err != nil {
		log.Printf("[POOL_GENERATION] ERROR: Batch pool generation failed for session %s (took %v): %v",
			sessionID, batchDuration, err)
		return fmt.Errorf("failed to batch generate pools: %w", err)
	}

	totalDuration := time.Since(sessionStartTime)
	log.Printf("[POOL_GENERATION] SUCCESS: Generated %d pools for session %s (total: %v, batch: %v)",
		len(poolConfigs), sessionID, totalDuration, batchDuration)
	log.Printf("[POOL_GENERATION] Performance - Avg per pool: %v, Skill: %s", 
		totalDuration/time.Duration(len(poolConfigs)), skillInfo.Name)

	return nil
}

// Helper methods to support the optimized pool generation
// Note: getBloomDistribution and getRecoveryBloomDistribution methods
// are already defined in session_service.go

// GetSkillCacheStats provides monitoring information about the skill cache
func (s *SessionService) GetSkillCacheStats() map[string]interface{} {
	// Validate service state
	if s == nil {
		return map[string]interface{}{
			"error": "session service is nil",
		}
	}

	log.Printf("[SKILL_CACHE] Retrieving skill cache statistics")
	skillCache := selection.GetSkillCache()
	if skillCache == nil {
		log.Printf("[SKILL_CACHE] ERROR: Skill cache is not initialized")
		return map[string]interface{}{
			"error": "skill cache is not initialized",
		}
	}

	stats := skillCache.GetCacheStats()
	if stats == nil {
		log.Printf("[SKILL_CACHE] ERROR: Failed to retrieve cache stats")
		return map[string]interface{}{
			"error": "failed to retrieve cache stats",
		}
	}

	log.Printf("[SKILL_CACHE] Current cache stats - entries: %v, total questions: %v",
		stats["total_entries"], stats["total_questions"])

	return stats
}

// InvalidateSkillCache allows manual cache invalidation for a specific skill
func (s *SessionService) InvalidateSkillCache(skillName string) {
	// Validate inputs and service state
	if s == nil {
		fmt.Printf("[SessionService] ERROR: Session service is nil\n")
		return
	}
	if skillName == "" {
		fmt.Printf("[SessionService] ERROR: Skill name cannot be empty\n")
		return
	}

	fmt.Printf("[SessionService] DEBUG: Invalidating skill cache for: %s\n", skillName)
	skillCache := selection.GetSkillCache()
	if skillCache == nil {
		fmt.Printf("[SessionService] ERROR: Skill cache is not initialized\n")
		return
	}

	skillCache.InvalidateSkillCache(skillName)
	fmt.Printf("[SessionService] DEBUG: Skill cache invalidated successfully for: %s\n", skillName)
}

// PrewarmSkillCache loads frequently used skills into cache proactively
func (s *SessionService) PrewarmSkillCache(ctx context.Context, skillNames []string) error {
	// Validate inputs and service state
	if s == nil {
		return fmt.Errorf("session service is nil")
	}
	if len(skillNames) == 0 {
		return fmt.Errorf("skillNames cannot be empty")
	}
	if s.QuestionRepo == nil {
		return fmt.Errorf("question repository is not initialized")
	}

	fmt.Printf("[SessionService] DEBUG: Starting cache prewarming for %d skills: %v\n", len(skillNames), skillNames)

	prewarmStartTime := time.Now()
	skillCache := selection.GetSkillCache()
	if skillCache == nil {
		return fmt.Errorf("skill cache is not initialized")
	}

	for i, skillName := range skillNames {
		skillStartTime := time.Now()

		// Check if already cached
		_, cacheHit := skillCache.GetCachedQuestions(skillName, "session_creation")
		if cacheHit {
			fmt.Printf("[SessionService] DEBUG: Skill '%s' (%d/%d) already cached - skipping\n",
				skillName, i+1, len(skillNames))
			continue
		}

		// Fetch and cache questions
		fmt.Printf("[SessionService] DEBUG: Prewarming cache for skill '%s' (%d/%d)\n",
			skillName, i+1, len(skillNames))

		questions, err := s.QuestionRepo.FindAll(ctx)
		if err != nil {
			fmt.Printf("[SessionService] WARNING: Failed to prewarm cache for skill '%s': %v\n", skillName, err)
			continue
		}

		skillCache.SetCachedQuestions(skillName, "session_creation", questions)
		skillDuration := time.Since(skillStartTime)

		fmt.Printf("[SessionService] DEBUG: Skill '%s' prewarmed - %d questions cached in %v\n",
			skillName, len(questions), skillDuration)
	}

	totalDuration := time.Since(prewarmStartTime)
	fmt.Printf("[SessionService] DEBUG: Cache prewarming COMPLETED for %d skills in %v\n",
		len(skillNames), totalDuration)

	return nil
}
