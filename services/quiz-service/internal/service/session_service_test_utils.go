// services/quiz-service/internal/service/session_service_test_utils.go
package service

import (
	"context"
	"fmt"
	"quiz-service/internal/selection"
	"time"
)

// TestSkillCachePerformance tests the performance of skill-based caching
func (s *SessionService) TestSkillCachePerformance(ctx context.Context, skillName string, testRuns int) map[string]interface{} {
	fmt.Printf("[TestUtils] DEBUG: Starting skill cache performance test\n")
	fmt.Printf("[TestUtils] DEBUG: Skill: '%s', Test runs: %d\n", skillName, testRuns)

	testStartTime := time.Now()

	// Clear any existing cache for this skill to start fresh
	s.InvalidateSkillCache(skillName)

	// Create test skill info (for potential future use)
	_ = &selection.SkillInfo{
		ID:   "test_skill_id",
		Name: skillName,
		Tags: []string{"test", "performance", "benchmark"},
	}

	// Test results storage
	var cacheMissTimes []time.Duration
	var cacheHitTimes []time.Duration
	var dbFetchTimes []time.Duration

	skillCache := selection.GetSkillCache()

	fmt.Printf("[TestUtils] DEBUG: Starting %d test runs\n", testRuns)

	for i := 0; i < testRuns; i++ {
		runStartTime := time.Now()

		// Check cache first to measure cache lookup time
		cacheStartTime := time.Now()
		cachedQuestions, cacheHit := skillCache.GetCachedQuestions(skillName, "session_creation")
		cacheLookupTime := time.Since(cacheStartTime)

		if cacheHit {
			// Cache hit - measure time
			cacheHitTimes = append(cacheHitTimes, cacheLookupTime)
			fmt.Printf("[TestUtils] DEBUG: Run %d - Cache HIT (%d questions, %v)\n",
				i+1, len(cachedQuestions), cacheLookupTime)
		} else {
			// Cache miss - fetch from database and cache
			cacheMissTimes = append(cacheMissTimes, cacheLookupTime)

			fmt.Printf("[TestUtils] DEBUG: Run %d - Cache MISS (%v cache check)\n", i+1, cacheLookupTime)

			// Simulate database fetch
			dbStartTime := time.Now()
			questions, err := s.QuestionRepo.FindAll(ctx)
			dbFetchTime := time.Since(dbStartTime)
			dbFetchTimes = append(dbFetchTimes, dbFetchTime)

			if err != nil {
				fmt.Printf("[TestUtils] ERROR: Database fetch failed in run %d: %v\n", i+1, err)
				continue
			}

			// Cache the questions
			cacheStoreStartTime := time.Now()
			skillCache.SetCachedQuestions(skillName, "session_creation", questions)
			cacheStoreTime := time.Since(cacheStoreStartTime)

			fmt.Printf("[TestUtils] DEBUG: Run %d - DB fetch: %v, Cache store: %v (%d questions)\n",
				i+1, dbFetchTime, cacheStoreTime, len(questions))
		}

		runDuration := time.Since(runStartTime)
		fmt.Printf("[TestUtils] DEBUG: Run %d completed in %v\n", i+1, runDuration)

		// Small delay between runs to simulate real usage patterns
		time.Sleep(10 * time.Millisecond)
	}

	// Calculate performance metrics
	totalTestDuration := time.Since(testStartTime)

	// Cache miss statistics
	avgCacheMissTime := time.Duration(0)
	if len(cacheMissTimes) > 0 {
		var total time.Duration
		for _, t := range cacheMissTimes {
			total += t
		}
		avgCacheMissTime = total / time.Duration(len(cacheMissTimes))
	}

	// Cache hit statistics
	avgCacheHitTime := time.Duration(0)
	if len(cacheHitTimes) > 0 {
		var total time.Duration
		for _, t := range cacheHitTimes {
			total += t
		}
		avgCacheHitTime = total / time.Duration(len(cacheHitTimes))
	}

	// Database fetch statistics
	avgDbFetchTime := time.Duration(0)
	if len(dbFetchTimes) > 0 {
		var total time.Duration
		for _, t := range dbFetchTimes {
			total += t
		}
		avgDbFetchTime = total / time.Duration(len(dbFetchTimes))
	}

	// Calculate performance improvement
	performanceGain := 0.0
	if avgCacheMissTime > 0 && avgCacheHitTime > 0 {
		performanceGain = float64(avgCacheMissTime-avgCacheHitTime) / float64(avgCacheMissTime) * 100
	}

	// Get final cache stats
	finalStats := s.GetSkillCacheStats()

	results := map[string]interface{}{
		"test_config": map[string]interface{}{
			"skill_name":          skillName,
			"test_runs":           testRuns,
			"total_test_duration": totalTestDuration.String(),
		},
		"cache_performance": map[string]interface{}{
			"cache_hits":           len(cacheHitTimes),
			"cache_misses":         len(cacheMissTimes),
			"hit_rate_percentage":  float64(len(cacheHitTimes)) / float64(testRuns) * 100,
			"avg_cache_hit_time":   avgCacheHitTime.String(),
			"avg_cache_miss_time":  avgCacheMissTime.String(),
			"performance_gain_pct": performanceGain,
		},
		"database_performance": map[string]interface{}{
			"db_fetches":            len(dbFetchTimes),
			"avg_db_fetch_time":     avgDbFetchTime.String(),
			"db_calls_saved":        testRuns - len(dbFetchTimes),
			"db_call_reduction_pct": float64(testRuns-len(dbFetchTimes)) / float64(testRuns) * 100,
		},
		"cache_stats": finalStats,
		"timestamp":   time.Now(),
	}

	fmt.Printf("[TestUtils] DEBUG: Performance test COMPLETED for skill '%s'\n", skillName)
	fmt.Printf("[TestUtils] DEBUG: Cache hits: %d/%d (%.1f%%), Performance gain: %.1f%%\n",
		len(cacheHitTimes), testRuns, float64(len(cacheHitTimes))/float64(testRuns)*100, performanceGain)

	return results
}

// SimulateSessionCreationLoad simulates multiple concurrent session creations
func (s *SessionService) SimulateSessionCreationLoad(ctx context.Context, skillName string, sessionCount int) map[string]interface{} {
	fmt.Printf("[TestUtils] DEBUG: Starting session creation load test\n")
	fmt.Printf("[TestUtils] DEBUG: Skill: '%s', Session count: %d\n", skillName, sessionCount)

	loadTestStartTime := time.Now()

	// Clear cache to start fresh
	s.InvalidateSkillCache(skillName)

	// Create test skill info
	testSkillInfo := &selection.SkillInfo{
		ID:   "load_test_skill",
		Name: skillName,
		Tags: []string{"load", "test", "performance"},
	}

	var sessionTimes []time.Duration
	var totalDbCalls int
	var cacheHits int
	var cacheMisses int

	fmt.Printf("[TestUtils] DEBUG: Creating %d test sessions\n", sessionCount)

	for i := 0; i < sessionCount; i++ {
		sessionStartTime := time.Now()
		sessionID := fmt.Sprintf("load_test_session_%d", i+1)

		fmt.Printf("[TestUtils] DEBUG: Creating session %d/%d - %s\n", i+1, sessionCount, sessionID)

		// Simulate pool generation (the main performance bottleneck)
		poolConfigs := []selection.PoolConfig{
			{Stage: "easy", Type: "initial", PoolSize: 50},
			{Stage: "easy", Type: "recovery", PoolSize: 25},
			{Stage: "medium", Type: "initial", PoolSize: 50},
			{Stage: "medium", Type: "recovery", PoolSize: 25},
			{Stage: "hard", Type: "initial", PoolSize: 50},
			{Stage: "hard", Type: "recovery", PoolSize: 25},
		}

		// Track cache hits/misses for this session
		skillCache := selection.GetSkillCache()
		_, cacheHit := skillCache.GetCachedQuestions(skillName, "session_creation")

		if cacheHit {
			cacheHits++
			fmt.Printf("[TestUtils] DEBUG: Session %d - Using CACHED data\n", i+1)
		} else {
			cacheMisses++
			totalDbCalls++
			fmt.Printf("[TestUtils] DEBUG: Session %d - Cache MISS, fetching from DB\n", i+1)
		}

		// Simulate batch pool generation
		err := s.poolManager.BatchGeneratePoolsFromCache(ctx, testSkillInfo, sessionID, poolConfigs)
		if err != nil {
			fmt.Printf("[TestUtils] ERROR: Session %d pool generation failed: %v\n", i+1, err)
			continue
		}

		sessionDuration := time.Since(sessionStartTime)
		sessionTimes = append(sessionTimes, sessionDuration)

		fmt.Printf("[TestUtils] DEBUG: Session %d completed in %v\n", i+1, sessionDuration)

		// Small delay to simulate realistic session creation patterns
		time.Sleep(5 * time.Millisecond)
	}

	// Calculate load test metrics
	totalLoadTestDuration := time.Since(loadTestStartTime)

	avgSessionTime := time.Duration(0)
	minSessionTime := time.Duration(0)
	maxSessionTime := time.Duration(0)

	if len(sessionTimes) > 0 {
		var total time.Duration
		minSessionTime = sessionTimes[0]
		maxSessionTime = sessionTimes[0]

		for _, t := range sessionTimes {
			total += t
			if t < minSessionTime {
				minSessionTime = t
			}
			if t > maxSessionTime {
				maxSessionTime = t
			}
		}
		avgSessionTime = total / time.Duration(len(sessionTimes))
	}

	// Calculate database call reduction
	maxPossibleDbCalls := sessionCount * 6 // 6 pools per session without caching
	dbCallReduction := float64(maxPossibleDbCalls-totalDbCalls) / float64(maxPossibleDbCalls) * 100

	results := map[string]interface{}{
		"load_test_config": map[string]interface{}{
			"skill_name":          skillName,
			"session_count":       sessionCount,
			"total_test_duration": totalLoadTestDuration.String(),
		},
		"session_performance": map[string]interface{}{
			"successful_sessions": len(sessionTimes),
			"avg_session_time":    avgSessionTime.String(),
			"min_session_time":    minSessionTime.String(),
			"max_session_time":    maxSessionTime.String(),
			"sessions_per_second": float64(len(sessionTimes)) / totalLoadTestDuration.Seconds(),
		},
		"cache_efficiency": map[string]interface{}{
			"cache_hits":            cacheHits,
			"cache_misses":          cacheMisses,
			"hit_rate_percentage":   float64(cacheHits) / float64(sessionCount) * 100,
			"total_db_calls":        totalDbCalls,
			"max_possible_db_calls": maxPossibleDbCalls,
			"db_call_reduction_pct": dbCallReduction,
		},
		"final_cache_stats": s.GetSkillCacheStats(),
		"timestamp":         time.Now(),
	}

	fmt.Printf("[TestUtils] DEBUG: Load test COMPLETED for %d sessions\n", sessionCount)
	fmt.Printf("[TestUtils] DEBUG: Cache efficiency - hits: %d/%d (%.1f%%), DB calls reduced by %.1f%%\n",
		cacheHits, sessionCount, float64(cacheHits)/float64(sessionCount)*100, dbCallReduction)

	return results
}

