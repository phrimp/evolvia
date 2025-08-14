// services/quiz-service/internal/handlers/cache_handler.go
package handlers

import (
	"fmt"
	"net/http"
	"quiz-service/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheHandler handles cache-related operations
type CacheHandler struct {
	SessionService *service.SessionService
}

// NewCacheHandler creates a new cache handler
func NewCacheHandler(sessionService *service.SessionService) *CacheHandler {
	// Validate input to prevent nil pointer panics
	if sessionService == nil {
		fmt.Printf("[CacheHandler] WARNING: Creating cache handler with nil session service\n")
	}
	
	return &CacheHandler{
		SessionService: sessionService,
	}
}

// GetSkillCacheStats returns comprehensive skill cache statistics
func (h *CacheHandler) GetSkillCacheStats(c *gin.Context) {
	// Validate handler state
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "cache handler is nil",
		})
		return
	}
	if h.SessionService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "session service is not initialized",
		})
		return
	}
	
	fmt.Printf("[CacheHandler] DEBUG: Skill cache stats requested at %v\n", time.Now().Format("15:04:05"))
	
	// Check admin access (simplified check - implement proper auth in production)
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		fmt.Printf("[CacheHandler] DEBUG: Unauthorized cache stats request - admin mode required\n")
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for cache statistics",
		})
		return
	}

	startTime := time.Now()
	stats := h.SessionService.GetSkillCacheStats()
	duration := time.Since(startTime)
	
	// Check if stats retrieval failed
	if stats == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve cache statistics",
		})
		return
	}
	
	fmt.Printf("[CacheHandler] DEBUG: Cache stats retrieved in %v\n", duration)

	response := gin.H{
		"cache_statistics": stats,
		"timestamp":        time.Now(),
		"retrieval_time":   duration.String(),
		"description":      "Skill-based question cache statistics for performance monitoring",
	}

	fmt.Printf("[CacheHandler] DEBUG: Returning cache stats - total entries: %v\n", stats["total_entries"])
	c.JSON(http.StatusOK, response)
}

// InvalidateSkillCache invalidates cache for a specific skill
func (h *CacheHandler) InvalidateSkillCache(c *gin.Context) {
	skillName := c.Param("skillName")
	
	fmt.Printf("[CacheHandler] DEBUG: Cache invalidation requested for skill: '%s'\n", skillName)
	
	// Check admin access
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		fmt.Printf("[CacheHandler] DEBUG: Unauthorized cache invalidation request for skill '%s'\n", skillName)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for cache invalidation",
		})
		return
	}

	if skillName == "" {
		fmt.Printf("[CacheHandler] DEBUG: Invalid cache invalidation request - skill name is empty\n")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Skill name is required",
		})
		return
	}

	// Get cache stats before invalidation
	statsBefore := h.SessionService.GetSkillCacheStats()
	entriesBefore := getEntryCount(statsBefore, skillName)
	
	startTime := time.Now()
	h.SessionService.InvalidateSkillCache(skillName)
	duration := time.Since(startTime)
	
	// Get cache stats after invalidation
	statsAfter := h.SessionService.GetSkillCacheStats()
	entriesAfter := getEntryCount(statsAfter, skillName)
	
	entriesRemoved := entriesBefore - entriesAfter
	
	fmt.Printf("[CacheHandler] DEBUG: Cache invalidation COMPLETED for skill '%s' - removed %d entries in %v\n", 
		skillName, entriesRemoved, duration)

	c.JSON(http.StatusOK, gin.H{
		"message":         fmt.Sprintf("Cache invalidated for skill: %s", skillName),
		"skill_name":      skillName,
		"entries_removed": entriesRemoved,
		"duration":        duration.String(),
		"timestamp":       time.Now(),
	})
}

// PrewarmSkillCache preloads frequently used skills into cache
func (h *CacheHandler) PrewarmSkillCache(c *gin.Context) {
	fmt.Printf("[CacheHandler] DEBUG: Cache prewarming requested\n")
	
	// Check admin access
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for cache prewarming",
		})
		return
	}

	var request struct {
		SkillNames []string `json:"skill_names" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("[CacheHandler] DEBUG: Invalid prewarming request: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if len(request.SkillNames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one skill name is required",
		})
		return
	}

	fmt.Printf("[CacheHandler] DEBUG: Prewarming requested for %d skills: %v\n", 
		len(request.SkillNames), request.SkillNames)

	startTime := time.Now()
	err := h.SessionService.PrewarmSkillCache(c.Request.Context(), request.SkillNames)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("[CacheHandler] ERROR: Cache prewarming failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to prewarm cache",
			"details": err.Error(),
		})
		return
	}

	fmt.Printf("[CacheHandler] DEBUG: Cache prewarming COMPLETED for %d skills in %v\n", 
		len(request.SkillNames), duration)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Cache prewarming completed",
		"skills":      request.SkillNames,
		"skill_count": len(request.SkillNames),
		"duration":    duration.String(),
		"timestamp":   time.Now(),
	})
}

// GetCacheHealth returns cache health and performance metrics
func (h *CacheHandler) GetCacheHealth(c *gin.Context) {
	fmt.Printf("[CacheHandler] DEBUG: Cache health check requested\n")
	
	startTime := time.Now()
	stats := h.SessionService.GetSkillCacheStats()
	healthCheckDuration := time.Since(startTime)

	totalEntries := 0
	totalQuestions := 0
	totalUsage := 0

	if entries, ok := stats["total_entries"].(int); ok {
		totalEntries = entries
	}
	if questions, ok := stats["total_questions"].(int); ok {
		totalQuestions = questions
	}
	if usage, ok := stats["total_usage"].(int); ok {
		totalUsage = usage
	}

	// Calculate health metrics
	avgQuestionsPerEntry := 0.0
	if totalEntries > 0 {
		avgQuestionsPerEntry = float64(totalQuestions) / float64(totalEntries)
	}

	avgUsagePerEntry := 0.0
	if totalEntries > 0 {
		avgUsagePerEntry = float64(totalUsage) / float64(totalEntries)
	}

	// Determine health status
	healthStatus := "healthy"
	warnings := []string{}

	if totalEntries == 0 {
		healthStatus = "warning"
		warnings = append(warnings, "No cache entries found")
	}

	if healthCheckDuration > 100*time.Millisecond {
		healthStatus = "warning" 
		warnings = append(warnings, fmt.Sprintf("Health check took %v (slow)", healthCheckDuration))
	}

	health := gin.H{
		"status":                  healthStatus,
		"total_entries":          totalEntries,
		"total_questions":        totalQuestions,
		"total_usage":            totalUsage,
		"avg_questions_per_entry": avgQuestionsPerEntry,
		"avg_usage_per_entry":    avgUsagePerEntry,
		"health_check_duration":  healthCheckDuration.String(),
		"warnings":               warnings,
		"timestamp":              time.Now(),
	}

	fmt.Printf("[CacheHandler] DEBUG: Cache health - status: %s, entries: %d, check duration: %v\n", 
		healthStatus, totalEntries, healthCheckDuration)

	c.JSON(http.StatusOK, gin.H{
		"cache_health": health,
	})
}

// GetCacheInfo returns detailed information about cache configuration
func (h *CacheHandler) GetCacheInfo(c *gin.Context) {
	fmt.Printf("[CacheHandler] DEBUG: Cache info requested\n")
	
	stats := h.SessionService.GetSkillCacheStats()
	
	info := gin.H{
		"cache_type":        "skill-based multi-session cache",
		"cache_key_format":  "skill_name_method",
		"ttl_hours":         stats["cache_ttl_hours"],
		"renew_on_use":      stats["renew_on_use"],
		"cleanup_interval":  "15 minutes",
		"current_stats":     stats,
		"description":       "Optimized caching system for quiz questions based on skills",
		"benefits": []string{
			"Eliminates redundant database calls",
			"Shares question data across sessions with same skill",
			"Automatic TTL renewal on access",
			"Periodic cleanup of expired entries",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"cache_info": info,
		"timestamp":  time.Now(),
	})
}

// Helper function to get entry count for a specific skill
func getEntryCount(stats map[string]interface{}, skillName string) int {
	if entriesBySkill, ok := stats["entries_by_skill"].(map[string]int); ok {
		return entriesBySkill[skillName]
	}
	return 0
}

// SetupCacheRoutes sets up cache management routes
func SetupCacheRoutes(router *gin.Engine, sessionService *service.SessionService) {
	cacheHandler := NewCacheHandler(sessionService)
	
	// Admin cache management routes
	admin := router.Group("/admin/cache")
	{
		admin.GET("/skill/stats", cacheHandler.GetSkillCacheStats)
		admin.DELETE("/skill/:skillName", cacheHandler.InvalidateSkillCache) 
		admin.POST("/skill/prewarm", cacheHandler.PrewarmSkillCache)
		admin.GET("/health", cacheHandler.GetCacheHealth)
		admin.GET("/info", cacheHandler.GetCacheInfo)
	}
	
	fmt.Printf("[CacheHandler] DEBUG: Cache management routes registered under /admin/cache\n")
}