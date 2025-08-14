// services/quiz-service/internal/handlers/test_handler.go
package handlers

import (
	"fmt"
	"net/http"
	"quiz-service/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// TestHandler handles performance testing operations
type TestHandler struct {
	SessionService *service.SessionService
}

// NewTestHandler creates a new test handler
func NewTestHandler(sessionService *service.SessionService) *TestHandler {
	return &TestHandler{
		SessionService: sessionService,
	}
}

// TestSkillCachePerformance runs skill cache performance tests
func (h *TestHandler) TestSkillCachePerformance(c *gin.Context) {
	fmt.Printf("[TestHandler] DEBUG: Skill cache performance test requested\n")
	
	// Check admin access
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for performance testing",
		})
		return
	}

	skillName := c.DefaultQuery("skill", "javascript")
	testRunsStr := c.DefaultQuery("runs", "10")
	
	testRuns, err := strconv.Atoi(testRunsStr)
	if err != nil || testRuns < 1 || testRuns > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid test runs parameter (must be 1-100)",
		})
		return
	}

	fmt.Printf("[TestHandler] DEBUG: Starting performance test - skill: '%s', runs: %d\n", skillName, testRuns)
	
	startTime := time.Now()
	results := h.SessionService.TestSkillCachePerformance(c.Request.Context(), skillName, testRuns)
	testDuration := time.Since(startTime)
	
	fmt.Printf("[TestHandler] DEBUG: Performance test completed in %v\n", testDuration)

	response := gin.H{
		"test_results":     results,
		"test_duration":    testDuration.String(),
		"timestamp":        time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// TestSessionCreationLoad runs session creation load tests
func (h *TestHandler) TestSessionCreationLoad(c *gin.Context) {
	fmt.Printf("[TestHandler] DEBUG: Session creation load test requested\n")
	
	// Check admin access
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for load testing",
		})
		return
	}

	skillName := c.DefaultQuery("skill", "javascript")
	sessionCountStr := c.DefaultQuery("sessions", "10")
	
	sessionCount, err := strconv.Atoi(sessionCountStr)
	if err != nil || sessionCount < 1 || sessionCount > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid session count parameter (must be 1-100)",
		})
		return
	}

	fmt.Printf("[TestHandler] DEBUG: Starting load test - skill: '%s', sessions: %d\n", skillName, sessionCount)
	
	startTime := time.Now()
	results := h.SessionService.SimulateSessionCreationLoad(c.Request.Context(), skillName, sessionCount)
	testDuration := time.Since(startTime)
	
	fmt.Printf("[TestHandler] DEBUG: Load test completed in %v\n", testDuration)

	response := gin.H{
		"load_test_results": results,
		"test_duration":     testDuration.String(),
		"timestamp":         time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// ComparePerformance compares cached vs non-cached performance
func (h *TestHandler) ComparePerformance(c *gin.Context) {
	fmt.Printf("[TestHandler] DEBUG: Performance comparison test requested\n")
	
	// Check admin access
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for performance testing",
		})
		return
	}

	skillName := c.DefaultQuery("skill", "javascript")
	testRunsStr := c.DefaultQuery("runs", "5")
	
	testRuns, err := strconv.Atoi(testRunsStr)
	if err != nil || testRuns < 1 || testRuns > 50 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid test runs parameter (must be 1-50)",
		})
		return
	}

	fmt.Printf("[TestHandler] DEBUG: Starting performance comparison - skill: '%s', runs: %d\n", skillName, testRuns)
	
	comparisonStartTime := time.Now()
	
	// Test 1: With caching (current implementation)
	fmt.Printf("[TestHandler] DEBUG: Testing WITH caching\n")
	cachedResults := h.SessionService.TestSkillCachePerformance(c.Request.Context(), skillName, testRuns)
	
	// Clear cache for non-cached test
	h.SessionService.InvalidateSkillCache(skillName)
	
	// Test 2: Load test to simulate multiple sessions
	fmt.Printf("[TestHandler] DEBUG: Testing session creation load\n")
	loadResults := h.SessionService.SimulateSessionCreationLoad(c.Request.Context(), skillName, testRuns)
	
	totalComparisonDuration := time.Since(comparisonStartTime)
	
	// Extract key metrics for comparison
	var cachedHitRate, dbCallReduction float64
	
	if cachePerf, ok := cachedResults["cache_performance"].(map[string]interface{}); ok {
		if hitRate, ok := cachePerf["hit_rate_percentage"].(float64); ok {
			cachedHitRate = hitRate
		}
	}
	
	if dbPerf, ok := cachedResults["database_performance"].(map[string]interface{}); ok {
		if reduction, ok := dbPerf["db_call_reduction_pct"].(float64); ok {
			dbCallReduction = reduction
		}
	}
	
	var loadAvgSessionTime float64
	if sessionPerf, ok := loadResults["session_performance"].(map[string]interface{}); ok {
		if avgTime, ok := sessionPerf["avg_session_time"].(string); ok {
			if duration, err := time.ParseDuration(avgTime); err == nil {
				loadAvgSessionTime = duration.Seconds()
			}
		}
	}
	
	comparison := gin.H{
		"test_configuration": gin.H{
			"skill_name":                skillName,
			"test_runs":                 testRuns,
			"total_comparison_duration": totalComparisonDuration.String(),
		},
		"performance_comparison": gin.H{
			"cache_hit_rate_percent":    cachedHitRate,
			"db_call_reduction_percent": dbCallReduction,
			"avg_session_time_seconds":  loadAvgSessionTime,
		},
		"detailed_results": gin.H{
			"cached_performance": cachedResults,
			"load_test_results":  loadResults,
		},
		"summary": gin.H{
			"caching_enabled":     cachedHitRate > 0,
			"performance_gained":  dbCallReduction > 0,
			"sessions_per_second": 1.0 / loadAvgSessionTime,
		},
		"timestamp": time.Now(),
	}
	
	fmt.Printf("[TestHandler] DEBUG: Performance comparison completed - hit rate: %.1f%%, DB reduction: %.1f%%\n", 
		cachedHitRate, dbCallReduction)

	c.JSON(http.StatusOK, comparison)
}

// GetTestInfo returns information about available tests
func (h *TestHandler) GetTestInfo(c *gin.Context) {
	info := gin.H{
		"available_tests": []gin.H{
			{
				"endpoint":    "GET /admin/test/cache/performance",
				"description": "Test skill cache performance with configurable runs",
				"parameters": []string{
					"skill (query param): skill name to test (default: javascript)",
					"runs (query param): number of test runs 1-100 (default: 10)",
				},
			},
			{
				"endpoint":    "GET /admin/test/session/load",
				"description": "Test session creation load with multiple concurrent sessions",
				"parameters": []string{
					"skill (query param): skill name to test (default: javascript)",
					"sessions (query param): number of sessions 1-100 (default: 10)",
				},
			},
			{
				"endpoint":    "GET /admin/test/performance/compare",
				"description": "Compare cached vs non-cached performance",
				"parameters": []string{
					"skill (query param): skill name to test (default: javascript)",
					"runs (query param): number of test runs 1-50 (default: 5)",
				},
			},
		},
		"requirements": []string{
			"Admin access required (X-Admin-Mode: true header)",
			"Database connection must be available",
			"Session service must be properly initialized",
		},
		"notes": []string{
			"Tests may temporarily affect cache state",
			"High run counts may impact system performance",
			"Results include detailed timing and cache statistics",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"test_info": info,
		"timestamp": time.Now(),
	})
}

// SetupTestRoutes sets up performance testing routes
func SetupTestRoutes(router *gin.Engine, sessionService *service.SessionService) {
	testHandler := NewTestHandler(sessionService)
	
	// Admin test routes
	admin := router.Group("/admin/test")
	{
		admin.GET("/info", testHandler.GetTestInfo)
		admin.GET("/cache/performance", testHandler.TestSkillCachePerformance)
		admin.GET("/session/load", testHandler.TestSessionCreationLoad)
		admin.GET("/performance/compare", testHandler.ComparePerformance)
	}
	
	fmt.Printf("[TestHandler] DEBUG: Performance testing routes registered under /admin/test\n")
}