package service

import (
	"context"
	"fmt"
	"log"
	"quiz-service/internal/adaptive"
	"quiz-service/internal/models"
	"sync"
	"time"
)

// ConfigBridge bridges database configuration to service configurations
// Minimal implementation to fix configuration inconsistency
type ConfigBridge struct {
	configService *ConfigService
	cache         *cachedConfigs
	mutex         sync.RWMutex
}

// cachedConfigs holds converted configurations with TTL
type cachedConfigs struct {
	bloomConfig    *models.BloomScoreConfig
	adaptiveConfig *adaptive.AdaptiveConfig
	lastUpdated    time.Time
	ttl            time.Duration
}

// NewConfigBridge creates a new configuration bridge
func NewConfigBridge(configService *ConfigService) *ConfigBridge {
	return &ConfigBridge{
		configService: configService,
		cache: &cachedConfigs{
			ttl: 5 * time.Minute, // 5-minute cache TTL
		},
	}
}

// GetBloomScoreConfig returns BloomScoreConfig from database configuration
func (cb *ConfigBridge) GetBloomScoreConfig(ctx context.Context) (*models.BloomScoreConfig, error) {
	cb.mutex.RLock()
	if cb.cache.bloomConfig != nil && time.Since(cb.cache.lastUpdated) < cb.cache.ttl {
		defer cb.mutex.RUnlock()
		return cb.cache.bloomConfig, nil
	}
	cb.mutex.RUnlock()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	globalConfig, err := cb.configService.GetDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	bloomConfig := cb.convertToBloomScoreConfig(globalConfig)

	// Update cache
	cb.cache.bloomConfig = bloomConfig
	cb.cache.lastUpdated = time.Now()

	log.Printf("[CONFIG_BRIDGE] Loaded BloomScoreConfig from database (cached for %v)", cb.cache.ttl)
	return bloomConfig, nil
}

// GetAdaptiveConfig returns AdaptiveConfig from database configuration
func (cb *ConfigBridge) GetAdaptiveConfig(ctx context.Context) (*adaptive.AdaptiveConfig, error) {
	cb.mutex.RLock()
	if cb.cache.adaptiveConfig != nil && time.Since(cb.cache.lastUpdated) < cb.cache.ttl {
		defer cb.mutex.RUnlock()
		return cb.cache.adaptiveConfig, nil
	}
	cb.mutex.RUnlock()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	globalConfig, err := cb.configService.GetDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	adaptiveConfig := cb.convertToAdaptiveConfig(globalConfig)

	// Update cache
	cb.cache.adaptiveConfig = adaptiveConfig
	cb.cache.lastUpdated = time.Now()

	log.Printf("[CONFIG_BRIDGE] Loaded AdaptiveConfig from database (cached for %v)", cb.cache.ttl)
	return adaptiveConfig, nil
}

// convertToBloomScoreConfig converts GlobalQuizConfig to BloomScoreConfig
func (cb *ConfigBridge) convertToBloomScoreConfig(globalConfig *models.GlobalQuizConfig) *models.BloomScoreConfig {
	bloomConfig := &models.BloomScoreConfig{
		BaseScores: map[string]int{
			"remember":   10,
			"understand": 15,
			"apply":      20,
			"analyze":    25,
			"evaluate":   30,
			"create":     35,
		},
		StageMultipliers: map[string]float64{
			"easy":   1.0,
			"medium": 1.2,
			"hard":   1.5,
		},
		InitialDistributions:  make(map[string]models.BloomDistribution),
		RecoveryDistributions: make(map[string]models.BloomDistribution),
	}

	// Convert stage configurations to BloomDistribution
	for stageName, stageConfig := range globalConfig.StageConfig {
		// Convert initial distribution
		bloomConfig.InitialDistributions[stageName] = models.BloomDistribution{
			Remember:   stageConfig.InitialBloomDistribution.Remember,
			Understand: stageConfig.InitialBloomDistribution.Understand,
			Apply:      stageConfig.InitialBloomDistribution.Apply,
			Analyze:    stageConfig.InitialBloomDistribution.Analyze,
			Evaluate:   stageConfig.InitialBloomDistribution.Evaluate,
			Create:     stageConfig.InitialBloomDistribution.Create,
		}

		// Convert recovery distribution
		bloomConfig.RecoveryDistributions[stageName] = models.BloomDistribution{
			Remember:   stageConfig.RecoveryBloomDistribution.Remember,
			Understand: stageConfig.RecoveryBloomDistribution.Understand,
			Apply:      stageConfig.RecoveryBloomDistribution.Apply,
			Analyze:    stageConfig.RecoveryBloomDistribution.Analyze,
			Evaluate:   stageConfig.RecoveryBloomDistribution.Evaluate,
			Create:     stageConfig.RecoveryBloomDistribution.Create,
		}
	}

	// Set relaxed distribution (average of all stages)
	bloomConfig.RelaxedDistribution = models.BloomDistribution{
		Remember:   0.2,
		Understand: 0.2,
		Apply:      0.2,
		Analyze:    0.2,
		Evaluate:   0.1,
		Create:     0.1,
	}

	return bloomConfig
}

// convertToAdaptiveConfig converts GlobalQuizConfig to AdaptiveConfig
func (cb *ConfigBridge) convertToAdaptiveConfig(globalConfig *models.GlobalQuizConfig) *adaptive.AdaptiveConfig {
	adaptiveConfig := &adaptive.AdaptiveConfig{
		MaxQuestions: globalConfig.MaxQuestions,
		StageConfigs: make(map[adaptive.Stage]adaptive.StageConfig),
	}

	// Convert stage configurations
	for stageName, enhancedStageConfig := range globalConfig.StageConfig {
		var stage adaptive.Stage
		switch stageName {
		case "easy":
			stage = adaptive.StageEasy
		case "medium":
			stage = adaptive.StageMedium
		case "hard":
			stage = adaptive.StageHard
		default:
			continue // Skip unknown stages
		}

		adaptiveConfig.StageConfigs[stage] = adaptive.StageConfig{
			InitialQuestions:  enhancedStageConfig.InitialQuestions,
			PassingThreshold:  enhancedStageConfig.PassingThreshold,
			RecoveryQuestions: enhancedStageConfig.RecoveryQuestions,
			RecoveryThreshold: enhancedStageConfig.RecoveryThreshold,
			BasePoints:        float64(enhancedStageConfig.BasePoints),
			RecoveryPoints:    float64(enhancedStageConfig.RecoveryPoints),
		}
	}

	return adaptiveConfig
}

// InvalidateCache forces reload of configuration from database
func (cb *ConfigBridge) InvalidateCache() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.cache.bloomConfig = nil
	cb.cache.adaptiveConfig = nil
	cb.cache.lastUpdated = time.Time{}

	log.Printf("[CONFIG_BRIDGE] Cache invalidated")
}

