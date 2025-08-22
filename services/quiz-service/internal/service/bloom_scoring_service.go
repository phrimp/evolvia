package service

import (
	"context"
	"fmt"
	"log"
	"quiz-service/internal/constants"
	"quiz-service/internal/models"
	"strings"
	"sync"
	"time"
)

// BloomScoringService provides centralized Bloom taxonomy scoring and distribution management
type BloomScoringService struct {
	configService  *ConfigService
	configBridge   *ConfigBridge // NEW: Bridge to database configuration
	config         *models.BloomScoreConfig
	cache          map[string]models.BloomDistribution
	cacheMutex     sync.RWMutex
	lastConfigLoad time.Time
	configTTL      time.Duration
}

// NewBloomScoringService creates a new centralized Bloom scoring service
func NewBloomScoringService(configService *ConfigService) *BloomScoringService {
	configBridge := NewConfigBridge(configService)
	
	service := &BloomScoringService{
		configService: configService,
		configBridge:  configBridge,
		config:        models.DefaultBloomScoreConfig(),
		cache:         make(map[string]models.BloomDistribution),
		configTTL:     5 * time.Minute, // Cache config for 5 minutes
	}

	// Load configuration from database via bridge
	if err := service.loadFromDatabase(context.Background()); err != nil {
		log.Printf("[BLOOM_SERVICE] Warning: Failed to load database config, using defaults: %v", err)
	}

	return service
}

// GetBloomDistribution returns the appropriate Bloom distribution for difficulty and context
func (bs *BloomScoringService) GetBloomDistribution(difficulty string, isRecovery bool) map[string]float64 {
	// Validate input
	if !constants.IsValidDifficultyLevel(difficulty) {
		log.Printf("[BLOOM_SERVICE] Invalid difficulty '%s', using easy", difficulty)
		difficulty = constants.DifficultyEasy
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s_%v", difficulty, isRecovery)
	bs.cacheMutex.RLock()
	if cached, exists := bs.cache[cacheKey]; exists {
		bs.cacheMutex.RUnlock()
		return cached.ToMap()
	}
	bs.cacheMutex.RUnlock()

	// Refresh config if needed
	if time.Since(bs.lastConfigLoad) > bs.configTTL {
		if err := bs.LoadFromConfig(context.Background()); err != nil {
			log.Printf("[BLOOM_SERVICE] Warning: Config refresh failed: %v", err)
		}
	}

	var distribution models.BloomDistribution

	if isRecovery {
		distribution = bs.config.RecoveryDistributions[difficulty]
	} else {
		distribution = bs.config.InitialDistributions[difficulty]
	}

	// Validate distribution
	if err := distribution.Validate(); err != nil {
		log.Printf("[BLOOM_SERVICE] Invalid distribution for %s (recovery=%v): %v, using relaxed",
			difficulty, isRecovery, err)
		distribution = bs.config.RelaxedDistribution
	}

	// Cache the result
	bs.cacheMutex.Lock()
	bs.cache[cacheKey] = distribution
	bs.cacheMutex.Unlock()

	log.Printf("[BLOOM_SERVICE] Generated distribution for %s (recovery=%v): %+v",
		difficulty, isRecovery, distribution)

	return distribution.ToMap()
}

// GetQuestionScore calculates the score for a question based on Bloom level and difficulty
func (bs *BloomScoringService) GetQuestionScore(bloomLevel, difficulty string) int {
	// Validate inputs
	if !constants.IsValidBloomLevel(bloomLevel) {
		log.Printf("[BLOOM_SERVICE] Invalid Bloom level '%s', using remember", bloomLevel)
		bloomLevel = constants.BloomLevelRemember
	}

	if !constants.IsValidDifficultyLevel(difficulty) {
		log.Printf("[BLOOM_SERVICE] Invalid difficulty '%s', using easy", difficulty)
		difficulty = constants.DifficultyEasy
	}

	// Get base score
	baseScore, exists := bs.config.BaseScores[bloomLevel]
	if !exists {
		log.Printf("[BLOOM_SERVICE] No base score for level '%s', using default 10", bloomLevel)
		baseScore = 10
	}

	// Get stage multiplier
	multiplier, exists := bs.config.StageMultipliers[difficulty]
	if !exists {
		log.Printf("[BLOOM_SERVICE] No multiplier for difficulty '%s', using 1.0", difficulty)
		multiplier = 1.0
	}

	score := int(float64(baseScore) * multiplier)

	log.Printf("[BLOOM_SERVICE] Calculated score: %s + %s = %d (base: %d, multiplier: %.1f)",
		bloomLevel, difficulty, score, baseScore, multiplier)

	return score
}

// GetRelaxedDistribution returns the fallback distribution when strict requirements cannot be met
func (bs *BloomScoringService) GetRelaxedDistribution() map[string]float64 {
	return bs.config.RelaxedDistribution.ToMap()
}

// GetQuestionScoresByStage calculates scores for all difficulty stages
func (bs *BloomScoringService) GetQuestionScoresByStage(bloomLevel string) map[string]int {
	scores := make(map[string]int)

	for _, difficulty := range constants.DifficultyLevels {
		scores[difficulty] = bs.GetQuestionScore(bloomLevel, difficulty)
	}

	return scores
}

// loadFromDatabase loads configuration using the ConfigBridge (NEW)
func (bs *BloomScoringService) loadFromDatabase(ctx context.Context) error {
	if bs.configBridge == nil {
		return fmt.Errorf("config bridge not available")
	}

	bloomConfig, err := bs.configBridge.GetBloomScoreConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bloom config from database: %w", err)
	}

	// Update the service configuration
	bs.config = bloomConfig
	bs.lastConfigLoad = time.Now()

	log.Printf("[BLOOM_SERVICE] Successfully loaded configuration from database")
	return nil
}

// ValidateDistribution checks if a distribution is valid
func (bs *BloomScoringService) ValidateDistribution(distMap map[string]float64) error {
	distribution := models.BloomDistributionFromMap(distMap)
	return distribution.Validate()
}

// LoadFromConfig loads Bloom configuration from the database
func (bs *BloomScoringService) LoadFromConfig(ctx context.Context) error {
	if bs.configService == nil {
		return fmt.Errorf("config service not available")
	}

	globalConfig, err := bs.configService.GetDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config: %w", err)
	}

	if globalConfig.StageConfig == nil {
		log.Printf("[BLOOM_SERVICE] No stage config in global config, keeping defaults")
		bs.lastConfigLoad = time.Now()
		return nil
	}

	// Update distributions from config
	for difficulty, stageConfig := range globalConfig.StageConfig {
		if !constants.IsValidDifficultyLevel(difficulty) {
			log.Printf("[BLOOM_SERVICE] Skipping invalid difficulty in config: %s", difficulty)
			continue
		}

		// Update initial distribution
		initialDist := models.BloomDistribution{
			Remember:   stageConfig.InitialBloomDistribution.Remember,
			Understand: stageConfig.InitialBloomDistribution.Understand,
			Apply:      stageConfig.InitialBloomDistribution.Apply,
			Analyze:    stageConfig.InitialBloomDistribution.Analyze,
			Evaluate:   stageConfig.InitialBloomDistribution.Evaluate,
			Create:     stageConfig.InitialBloomDistribution.Create,
		}

		if err := initialDist.Validate(); err != nil {
			log.Printf("[BLOOM_SERVICE] Invalid initial distribution in config for %s: %v", difficulty, err)
		} else {
			bs.config.InitialDistributions[difficulty] = initialDist
		}

		// Update recovery distribution
		recoveryDist := models.BloomDistribution{
			Remember:   stageConfig.RecoveryBloomDistribution.Remember,
			Understand: stageConfig.RecoveryBloomDistribution.Understand,
			Apply:      stageConfig.RecoveryBloomDistribution.Apply,
			Analyze:    stageConfig.RecoveryBloomDistribution.Analyze,
			Evaluate:   stageConfig.RecoveryBloomDistribution.Evaluate,
			Create:     stageConfig.RecoveryBloomDistribution.Create,
		}

		if err := recoveryDist.Validate(); err != nil {
			log.Printf("[BLOOM_SERVICE] Invalid recovery distribution in config for %s: %v", difficulty, err)
		} else {
			bs.config.RecoveryDistributions[difficulty] = recoveryDist
		}
	}

	// Clear cache to force reload
	bs.cacheMutex.Lock()
	bs.cache = make(map[string]models.BloomDistribution)
	bs.cacheMutex.Unlock()

	bs.lastConfigLoad = time.Now()
	log.Printf("[BLOOM_SERVICE] Successfully loaded configuration from database")

	return nil
}

// GetConfiguredDistribution retrieves distribution from database config
func (bs *BloomScoringService) GetConfiguredDistribution(difficulty string, isRecovery bool) (map[string]float64, error) {
	if err := bs.LoadFromConfig(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	var distribution models.BloomDistribution

	if isRecovery {
		distribution = bs.config.RecoveryDistributions[difficulty]
	} else {
		distribution = bs.config.InitialDistributions[difficulty]
	}

	if err := distribution.Validate(); err != nil {
		return nil, fmt.Errorf("invalid distribution: %w", err)
	}

	return distribution.ToMap(), nil
}

// GetHardcodedFallback returns hardcoded distributions as final fallback
func (bs *BloomScoringService) GetHardcodedFallback(difficulty string, isRecovery bool) map[string]float64 {
	defaultConfig := models.DefaultBloomScoreConfig()

	var distribution models.BloomDistribution

	if isRecovery {
		if dist, exists := defaultConfig.RecoveryDistributions[difficulty]; exists {
			distribution = dist
		} else {
			distribution = defaultConfig.RelaxedDistribution
		}
	} else {
		if dist, exists := defaultConfig.InitialDistributions[difficulty]; exists {
			distribution = dist
		} else {
			distribution = defaultConfig.RelaxedDistribution
		}
	}

	return distribution.ToMap()
}

// GetBaseScores returns the base scores for all Bloom levels
func (bs *BloomScoringService) GetBaseScores() map[string]int {
	return bs.config.BaseScores
}

// GetStageMultipliers returns the stage multipliers for all difficulty levels
func (bs *BloomScoringService) GetStageMultipliers() map[string]float64 {
	return bs.config.StageMultipliers
}

// ClearCache clears the internal distribution cache
func (bs *BloomScoringService) ClearCache() {
	bs.cacheMutex.Lock()
	bs.cache = make(map[string]models.BloomDistribution)
	bs.cacheMutex.Unlock()
	log.Printf("[BLOOM_SERVICE] Cache cleared")
}

// GetCustomBloomDistribution creates a custom distribution targeting specific Bloom levels
func (bs *BloomScoringService) GetCustomBloomDistribution(targetBlooms []string) map[string]float64 {
	if len(targetBlooms) == 0 {
		log.Printf("[BLOOM_SERVICE] No target Bloom levels provided, using relaxed distribution")
		return bs.GetRelaxedDistribution()
	}

	// Start with a base distribution
	dist := map[string]float64{
		constants.BloomLevelRemember:   0.1,
		constants.BloomLevelUnderstand: 0.1,
		constants.BloomLevelApply:      0.1,
		constants.BloomLevelAnalyze:    0.1,
		constants.BloomLevelEvaluate:   0.1,
		constants.BloomLevelCreate:     0.1,
	}

	// Distribute additional weight among target levels
	if len(targetBlooms) > 0 {
		targetWeight := 0.4 / float64(len(targetBlooms)) // 40% allocated to target levels
		for _, target := range targetBlooms {
			normalizedTarget := strings.ToLower(target)
			if constants.IsValidBloomLevel(normalizedTarget) {
				dist[normalizedTarget] += targetWeight
			} else {
				log.Printf("[BLOOM_SERVICE] Invalid target Bloom level '%s', skipping", target)
			}
		}
	}

	// Normalize to ensure sum equals 1.0
	total := 0.0
	for _, v := range dist {
		total += v
	}
	if total > 0 {
		for k := range dist {
			dist[k] = dist[k] / total
		}
	}

	log.Printf("[BLOOM_SERVICE] Generated custom distribution for targets %v: %+v", targetBlooms, dist)
	return dist
}

// GetCacheStatus returns cache statistics for monitoring
func (bs *BloomScoringService) GetCacheStatus() map[string]interface{} {
	bs.cacheMutex.RLock()
	defer bs.cacheMutex.RUnlock()

	return map[string]interface{}{
		"cache_size":         len(bs.cache),
		"last_config_load":   bs.lastConfigLoad,
		"config_ttl_minutes": bs.configTTL.Minutes(),
		"cache_keys": func() []string {
			var keys []string
			for k := range bs.cache {
				keys = append(keys, k)
			}
			return keys
		}(),
	}
}

