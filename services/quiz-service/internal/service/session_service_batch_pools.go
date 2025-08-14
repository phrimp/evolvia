// services/quiz-service/internal/service/session_service_batch_pools.go
package service

import (
	"context"
	"fmt"
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

	fmt.Printf("[SessionService] DEBUG: Starting optimized pool generation for session %s\n", sessionID)

	sessionStartTime := time.Now()
	skillInfo := s.getSkillInfoFromSession(session)
	if skillInfo == nil {
		fmt.Printf("[SessionService] ERROR: Skill information not found for session %s\n", sessionID)
		return fmt.Errorf("skill information not found for session")
	}

	fmt.Printf("[SessionService] DEBUG: Retrieved skill info for session %s - skill: '%s' (ID: %s), tags: %v\n",
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

	fmt.Printf("[SessionService] DEBUG: Created %d pool configurations for session %s\n", len(poolConfigs), sessionID)

	// Generate all pools in a single batch operation using skill cache
	batchStartTime := time.Now()
	err := s.poolManager.BatchGeneratePoolsFromCache(ctx, skillInfo, sessionID, poolConfigs)
	batchDuration := time.Since(batchStartTime)

	if err != nil {
		fmt.Printf("[SessionService] ERROR: Batch pool generation failed for session %s: %v (took %v)\n",
			sessionID, err, batchDuration)
		return fmt.Errorf("failed to batch generate pools: %w", err)
	}

	totalDuration := time.Since(sessionStartTime)
	fmt.Printf("[SessionService] DEBUG: Successfully generated %d pools for session %s in %v (batch: %v)\n",
		len(poolConfigs), sessionID, totalDuration, batchDuration)

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

	fmt.Printf("[SessionService] DEBUG: Retrieving skill cache statistics\n")
	skillCache := selection.GetSkillCache()
	if skillCache == nil {
		fmt.Printf("[SessionService] ERROR: Skill cache is not initialized\n")
		return map[string]interface{}{
			"error": "skill cache is not initialized",
		}
	}

	stats := skillCache.GetCacheStats()
	if stats == nil {
		fmt.Printf("[SessionService] ERROR: Failed to retrieve cache stats\n")
		return map[string]interface{}{
			"error": "failed to retrieve cache stats",
		}
	}

	fmt.Printf("[SessionService] DEBUG: Current cache stats - entries: %v, total questions: %v\n",
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
