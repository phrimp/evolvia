// services/quiz-service/internal/selection/skill_cache.go
package selection

import (
	"fmt"
	"quiz-service/internal/models"
	"strings"
	"sync"
	"time"
)

// SkillQuestionCache represents a skill-based cache entry
type SkillQuestionCache struct {
	SkillName  string            `json:"skill_name"`
	Questions  []models.Question `json:"questions"`
	TotalCount int               `json:"total_count"`
	CreatedAt  time.Time         `json:"created_at"`
	LastUsed   time.Time         `json:"last_used"`
	UsageCount int               `json:"usage_count"`
	Method     string            `json:"method"` // "session_creation", "question_selection", etc.
}

// SkillCacheManager handles skill-based question caching across sessions
type SkillCacheManager struct {
	cache      map[string]*SkillQuestionCache
	mutex      sync.RWMutex
	TTL        time.Duration // 1 hour default
	renewOnUse bool          // renew TTL when cache is accessed
}

// Global skill cache instance
var skillCacheManager *SkillCacheManager

func init() {
	skillCacheManager = &SkillCacheManager{
		cache:      make(map[string]*SkillQuestionCache),
		TTL:        1 * time.Hour, // Fixed 1 hour as requested
		renewOnUse: true,          // Renew lifetime when used
	}

	// Start cleanup goroutine
	go skillCacheManager.startCleanupRoutine()
}

// GetSkillCache returns the global skill cache manager
func GetSkillCache() *SkillCacheManager {
	return skillCacheManager
}

// GenerateCacheKey creates cache key in format: skill_name_method
// Example: "javascript_session_creation", "python_question_selection"
func (scm *SkillCacheManager) GenerateCacheKey(skillName, method string) string {
	// Sanitize skill name and method for cache key
	sanitizedSkillName := sanitizeForCacheKey(skillName)
	sanitizedMethod := sanitizeForCacheKey(method)
	return fmt.Sprintf("%s_%s", sanitizedSkillName, sanitizedMethod)
}

// GetCachedQuestions retrieves questions from skill cache
func (scm *SkillCacheManager) GetCachedQuestions(skillName, method string) ([]models.Question, bool) {
	scm.mutex.RLock()
	defer scm.mutex.RUnlock()

	cacheKey := scm.GenerateCacheKey(skillName, method)
	fmt.Printf("[SkillCache] DEBUG: Attempting cache lookup for key: %s\n", cacheKey)

	entry, exists := scm.cache[cacheKey]

	if !exists {
		fmt.Printf("[SkillCache] DEBUG: Cache MISS - key '%s' not found in cache\n", cacheKey)
		return nil, false
	}

	fmt.Printf("[SkillCache] DEBUG: Cache entry found for key '%s', created: %v, last used: %v\n",
		cacheKey, entry.CreatedAt.Format("15:04:05"), entry.LastUsed.Format("15:04:05"))

	// Check if cache entry has expired
	if scm.isExpired(entry) {
		fmt.Printf("[SkillCache] DEBUG: Cache entry EXPIRED for key '%s' - removing from cache\n", cacheKey)
		// Remove expired entry
		delete(scm.cache, cacheKey)
		return nil, false
	}

	// Renew cache lifetime if enabled
	if scm.renewOnUse {
		oldLastUsed := entry.LastUsed
		entry.LastUsed = time.Now()
		entry.UsageCount++
		fmt.Printf("[SkillCache] DEBUG: Cache lifetime RENEWED for key '%s' - last used updated from %v to %v, usage count: %d\n",
			cacheKey, oldLastUsed.Format("15:04:05"), entry.LastUsed.Format("15:04:05"), entry.UsageCount)
	}

	fmt.Printf("[SkillCache] DEBUG: Cache HIT - returning %d questions for key '%s'\n", len(entry.Questions), cacheKey)
	return entry.Questions, true
}

// SetCachedQuestions stores questions in skill cache
func (scm *SkillCacheManager) SetCachedQuestions(skillName, method string, questions []models.Question) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()

	cacheKey := scm.GenerateCacheKey(skillName, method)
	now := time.Now()

	fmt.Printf("[SkillCache] DEBUG: Storing %d questions in cache for key '%s'\n", len(questions), cacheKey)
	fmt.Printf("[SkillCache] DEBUG: Cache entry details - skill: '%s', method: '%s', TTL: %v\n",
		skillName, method, scm.TTL)

	scm.cache[cacheKey] = &SkillQuestionCache{
		SkillName:  skillName,
		Questions:  questions,
		TotalCount: len(questions),
		CreatedAt:  now,
		LastUsed:   now,
		UsageCount: 1,
		Method:     method,
	}

	fmt.Printf("[SkillCache] DEBUG: Cache entry STORED successfully for key '%s' - expires at: %v\n",
		cacheKey, now.Add(scm.TTL).Format("15:04:05"))
	fmt.Printf("[SkillCache] DEBUG: Current cache size: %d entries\n", len(scm.cache))
}

// InvalidateSkillCache removes all cache entries for a specific skill
func (scm *SkillCacheManager) InvalidateSkillCache(skillName string) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()

	keysToDelete := make([]string, 0)
	for key, entry := range scm.cache {
		if entry.SkillName == skillName {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(scm.cache, key)
	}
}

// GetCacheStats returns comprehensive cache statistics
func (scm *SkillCacheManager) GetCacheStats() map[string]interface{} {
	scm.mutex.RLock()
	defer scm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_entries":     len(scm.cache),
		"cache_ttl_hours":   scm.TTL.Hours(),
		"renew_on_use":      scm.renewOnUse,
		"entries_by_skill":  make(map[string]int),
		"entries_by_method": make(map[string]int),
		"total_questions":   0,
		"total_usage":       0,
	}

	skillCounts := make(map[string]int)
	methodCounts := make(map[string]int)
	totalQuestions := 0
	totalUsage := 0

	for _, entry := range scm.cache {
		skillCounts[entry.SkillName]++
		methodCounts[entry.Method]++
		totalQuestions += entry.TotalCount
		totalUsage += entry.UsageCount
	}

	stats["entries_by_skill"] = skillCounts
	stats["entries_by_method"] = methodCounts
	stats["total_questions"] = totalQuestions
	stats["total_usage"] = totalUsage

	return stats
}

// ClearExpiredEntries removes expired cache entries
func (scm *SkillCacheManager) ClearExpiredEntries() int {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()

	expiredKeys := make([]string, 0)
	for key, entry := range scm.cache {
		if scm.isExpired(entry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(scm.cache, key)
	}

	return len(expiredKeys)
}

// isExpired checks if cache entry has expired (internal use)
func (scm *SkillCacheManager) isExpired(entry *SkillQuestionCache) bool {
	// Use LastUsed if renewOnUse is enabled, otherwise use CreatedAt
	referenceTime := entry.CreatedAt
	if scm.renewOnUse {
		referenceTime = entry.LastUsed
	}

	return time.Since(referenceTime) > scm.TTL
}

// startCleanupRoutine runs periodic cleanup of expired entries
func (scm *SkillCacheManager) startCleanupRoutine() {
	ticker := time.NewTicker(15 * time.Minute) // Cleanup every 15 minutes
	defer ticker.Stop()

	fmt.Printf("[SkillCache] DEBUG: Cleanup routine STARTED - will run every 15 minutes\n")

	for {
		select {
		case <-ticker.C:
			fmt.Printf("[SkillCache] DEBUG: Starting periodic cleanup routine\n")
			startTime := time.Now()
			cleared := scm.ClearExpiredEntries()
			duration := time.Since(startTime)

			if cleared > 0 {
				fmt.Printf("[SkillCache] DEBUG: Cleanup COMPLETED - cleared %d expired entries in %v\n", cleared, duration)
			} else {
				fmt.Printf("[SkillCache] DEBUG: Cleanup COMPLETED - no expired entries found (took %v)\n", duration)
			}
		}
	}
}

// sanitizeForCacheKey sanitizes strings for use in cache keys
func sanitizeForCacheKey(input string) string {
	// Replace spaces and special characters with underscores
	result := strings.ReplaceAll(input, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ToLower(result)
	return result
}

// GetCacheInfo returns detailed information about specific cache entry
func (scm *SkillCacheManager) GetCacheInfo(skillName, method string) (*SkillQuestionCache, bool) {
	scm.mutex.RLock()
	defer scm.mutex.RUnlock()

	cacheKey := scm.GenerateCacheKey(skillName, method)
	entry, exists := scm.cache[cacheKey]

	if !exists || scm.isExpired(entry) {
		return nil, false
	}

	// Return a copy to avoid external modification
	entryCopy := *entry
	return &entryCopy, true
}

// SetTTL updates the cache TTL duration
func (scm *SkillCacheManager) SetTTL(duration time.Duration) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()
	scm.TTL = duration
}

// SetRenewOnUse enables/disables cache renewal on access
func (scm *SkillCacheManager) SetRenewOnUse(enable bool) {
	scm.mutex.Lock()
	defer scm.mutex.Unlock()
	scm.renewOnUse = enable
}

