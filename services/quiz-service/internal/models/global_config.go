package models

import (
	"fmt"
	"time"
)

// BloomDistributionConfig defines Bloom taxonomy level distributions
type BloomDistributionConfig struct {
	Remember   float64 `bson:"remember" json:"remember"`
	Understand float64 `bson:"understand" json:"understand"`
	Apply      float64 `bson:"apply" json:"apply"`
	Analyze    float64 `bson:"analyze" json:"analyze"`
	Evaluate   float64 `bson:"evaluate" json:"evaluate"`
	Create     float64 `bson:"create" json:"create"`
}

// TagWeightConfig defines weights for different tag categories
type TagWeightConfig struct {
	PrimaryWeight   float64 `bson:"primary_weight" json:"primary_weight"`
	SecondaryWeight float64 `bson:"secondary_weight" json:"secondary_weight"`
	RelatedWeight   float64 `bson:"related_weight" json:"related_weight"`
	ExactMatchBonus float64 `bson:"exact_match_bonus" json:"exact_match_bonus"`
}

// PoolConfig defines question pool generation settings
type PoolConfig struct {
	InitialPoolSize    int `bson:"initial_pool_size" json:"initial_pool_size"`
	RecoveryPoolSize   int `bson:"recovery_pool_size" json:"recovery_pool_size"`
	BufferSize         int `bson:"buffer_size" json:"buffer_size"`
	MinQuestionsPerTag int `bson:"min_questions_per_tag" json:"min_questions_per_tag"`
}

// ScoringConfig defines scoring thresholds and criteria
type ScoringConfig struct {
	BadgeThresholds struct {
		Expert       float64 `bson:"expert" json:"expert"`
		Proficient   float64 `bson:"proficient" json:"proficient"`
		Intermediate float64 `bson:"intermediate" json:"intermediate"`
	} `bson:"badge_thresholds" json:"badge_thresholds"`

	DifficultyThresholds struct {
		EasyToMedium  float64 `bson:"easy_to_medium" json:"easy_to_medium"`
		MediumToHard  float64 `bson:"medium_to_hard" json:"medium_to_hard"`
		MasteryEasy   int     `bson:"mastery_easy" json:"mastery_easy"`
		MasteryMedium int     `bson:"mastery_medium" json:"mastery_medium"`
	} `bson:"difficulty_thresholds" json:"difficulty_thresholds"`

	RecommendationThresholds struct {
		LowPerformance    float64 `bson:"low_performance" json:"low_performance"`
		MediumPerformance float64 `bson:"medium_performance" json:"medium_performance"`
		HighPerformance   float64 `bson:"high_performance" json:"high_performance"`
	} `bson:"recommendation_thresholds" json:"recommendation_thresholds"`
}

// CacheConfig defines caching settings
type CacheConfig struct {
	AnswerCacheRetentionMinutes int `bson:"answer_cache_retention_minutes" json:"answer_cache_retention_minutes"`
	PoolCacheTTLMinutes         int `bson:"pool_cache_ttl_minutes" json:"pool_cache_ttl_minutes"`
	MaxCachedPools              int `bson:"max_cached_pools" json:"max_cached_pools"`
}

// EnhancedStageConfig extends StageConfig with Bloom distributions
type EnhancedStageConfig struct {
	StageConfig
	InitialBloomDistribution  BloomDistributionConfig `bson:"initial_bloom_distribution" json:"initial_bloom_distribution"`
	RecoveryBloomDistribution BloomDistributionConfig `bson:"recovery_bloom_distribution" json:"recovery_bloom_distribution"`
}

// GlobalQuizConfig defines comprehensive global configuration for all quiz sessions
type GlobalQuizConfig struct {
	ID                   string                         `bson:"_id,omitempty" json:"id"`
	Name                 string                         `bson:"name" json:"name"`
	Description          string                         `bson:"description" json:"description"`
	StageConfig          map[string]EnhancedStageConfig `bson:"stage_config" json:"stage_config"`
	TotalDurationSeconds int                            `bson:"total_duration_seconds" json:"total_duration_seconds"`
	MaxQuestions         int                            `bson:"max_questions" json:"max_questions"`

	// New configurable components
	TagWeights    TagWeightConfig `bson:"tag_weights" json:"tag_weights"`
	PoolConfig    PoolConfig      `bson:"pool_config" json:"pool_config"`
	ScoringConfig ScoringConfig   `bson:"scoring_config" json:"scoring_config"`
	CacheConfig   CacheConfig     `bson:"cache_config" json:"cache_config"`

	// Metadata
	IsDefault bool      `bson:"is_default" json:"is_default"`
	Status    string    `bson:"status" json:"status"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// DefaultGlobalConfig creates a default configuration with comprehensive settings
func DefaultGlobalConfig() *GlobalQuizConfig {
	return &GlobalQuizConfig{
		Name:        "Default Adaptive Configuration",
		Description: "Default configuration for adaptive quiz sessions with Bloom taxonomy support",
		StageConfig: map[string]EnhancedStageConfig{
			"easy": {
				StageConfig: StageConfig{
					InitialQuestions:  5,
					PassingThreshold:  0.6,
					RecoveryQuestions: 3,
					RecoveryThreshold: 0.5,
					BasePoints:        10,
					RecoveryPoints:    5,
				},
				InitialBloomDistribution: BloomDistributionConfig{
					Remember:   0.5,
					Understand: 0.3,
					Apply:      0.2,
					Analyze:    0.0,
					Evaluate:   0.0,
					Create:     0.0,
				},
				RecoveryBloomDistribution: BloomDistributionConfig{
					Remember:   0.6,
					Understand: 0.3,
					Apply:      0.1,
					Analyze:    0.0,
					Evaluate:   0.0,
					Create:     0.0,
				},
			},
			"medium": {
				StageConfig: StageConfig{
					InitialQuestions:  5,
					PassingThreshold:  0.7,
					RecoveryQuestions: 3,
					RecoveryThreshold: 0.6,
					BasePoints:        15,
					RecoveryPoints:    8,
				},
				InitialBloomDistribution: BloomDistributionConfig{
					Remember:   0.25,
					Understand: 0.35,
					Apply:      0.25,
					Analyze:    0.15,
					Evaluate:   0.0,
					Create:     0.0,
				},
				RecoveryBloomDistribution: BloomDistributionConfig{
					Remember:   0.4,
					Understand: 0.3,
					Apply:      0.2,
					Analyze:    0.1,
					Evaluate:   0.0,
					Create:     0.0,
				},
			},
			"hard": {
				StageConfig: StageConfig{
					InitialQuestions:  5,
					PassingThreshold:  0.8,
					RecoveryQuestions: 3,
					RecoveryThreshold: 0.7,
					BasePoints:        20,
					RecoveryPoints:    12,
				},
				InitialBloomDistribution: BloomDistributionConfig{
					Remember:   0.15,
					Understand: 0.2,
					Apply:      0.25,
					Analyze:    0.2,
					Evaluate:   0.15,
					Create:     0.05,
				},
				RecoveryBloomDistribution: BloomDistributionConfig{
					Remember:   0.25,
					Understand: 0.25,
					Apply:      0.25,
					Analyze:    0.15,
					Evaluate:   0.1,
					Create:     0.0,
				},
			},
		},
		TotalDurationSeconds: 3600, // 60 minutes
		MaxQuestions:         20,

		// Tag weight configuration
		TagWeights: TagWeightConfig{
			PrimaryWeight:   1.0,
			SecondaryWeight: 0.7,
			RelatedWeight:   0.3,
			ExactMatchBonus: 0.2,
		},

		// Pool generation settings
		PoolConfig: PoolConfig{
			InitialPoolSize:    50,
			RecoveryPoolSize:   25,
			BufferSize:         15,
			MinQuestionsPerTag: 5,
		},

		// Scoring and badge thresholds
		ScoringConfig: ScoringConfig{
			BadgeThresholds: struct {
				Expert       float64 `bson:"expert" json:"expert"`
				Proficient   float64 `bson:"proficient" json:"proficient"`
				Intermediate float64 `bson:"intermediate" json:"intermediate"`
			}{
				Expert:       0.9,
				Proficient:   0.75,
				Intermediate: 0.6,
			},
			DifficultyThresholds: struct {
				EasyToMedium  float64 `bson:"easy_to_medium" json:"easy_to_medium"`
				MediumToHard  float64 `bson:"medium_to_hard" json:"medium_to_hard"`
				MasteryEasy   int     `bson:"mastery_easy" json:"mastery_easy"`
				MasteryMedium int     `bson:"mastery_medium" json:"mastery_medium"`
			}{
				EasyToMedium:  0.8,
				MediumToHard:  0.85,
				MasteryEasy:   3,
				MasteryMedium: 2,
			},
			RecommendationThresholds: struct {
				LowPerformance    float64 `bson:"low_performance" json:"low_performance"`
				MediumPerformance float64 `bson:"medium_performance" json:"medium_performance"`
				HighPerformance   float64 `bson:"high_performance" json:"high_performance"`
			}{
				LowPerformance:    0.5,
				MediumPerformance: 0.7,
				HighPerformance:   0.85,
			},
		},

		// Cache configuration
		CacheConfig: CacheConfig{
			AnswerCacheRetentionMinutes: 30,
			PoolCacheTTLMinutes:         60,
			MaxCachedPools:              100,
		},

		IsDefault: true,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Validate validates the global configuration
func (gc *GlobalQuizConfig) Validate() error {
	if gc.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(gc.StageConfig) == 0 {
		return fmt.Errorf("stage configuration is required")
	}

	requiredStages := []string{"easy", "medium", "hard"}
	for _, stage := range requiredStages {
		if _, exists := gc.StageConfig[stage]; !exists {
			return fmt.Errorf("missing stage configuration for: %s", stage)
		}
	}

	return nil
}
