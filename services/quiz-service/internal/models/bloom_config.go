package models

import (
	"fmt"
	"quiz-service/internal/constants"
)

// BloomDistribution represents a complete Bloom taxonomy distribution
type BloomDistribution struct {
	Remember   float64 `json:"remember" bson:"remember"`
	Understand float64 `json:"understand" bson:"understand"`
	Apply      float64 `json:"apply" bson:"apply"`
	Analyze    float64 `json:"analyze" bson:"analyze"`
	Evaluate   float64 `json:"evaluate" bson:"evaluate"`
	Create     float64 `json:"create" bson:"create"`
}

// BloomScoreConfig contains all Bloom scoring configuration
type BloomScoreConfig struct {
	// Base scores for each Bloom taxonomy level
	BaseScores map[string]int `json:"base_scores" bson:"base_scores"`

	// Stage multipliers for difficulty levels
	StageMultipliers map[string]float64 `json:"stage_multipliers" bson:"stage_multipliers"`

	// Initial distributions for each difficulty level
	InitialDistributions map[string]BloomDistribution `json:"initial_distributions" bson:"initial_distributions"`

	// Recovery distributions for each difficulty level
	RecoveryDistributions map[string]BloomDistribution `json:"recovery_distributions" bson:"recovery_distributions"`

	// Relaxed distribution used as fallback
	RelaxedDistribution BloomDistribution `json:"relaxed_distribution" bson:"relaxed_distribution"`
}

// ToMap converts BloomDistribution to map[string]float64 for compatibility
func (bd *BloomDistribution) ToMap() map[string]float64 {
	return map[string]float64{
		constants.BloomLevelRemember:   bd.Remember,
		constants.BloomLevelUnderstand: bd.Understand,
		constants.BloomLevelApply:      bd.Apply,
		constants.BloomLevelAnalyze:    bd.Analyze,
		constants.BloomLevelEvaluate:   bd.Evaluate,
		constants.BloomLevelCreate:     bd.Create,
	}
}

// FromMap creates BloomDistribution from map[string]float64
func BloomDistributionFromMap(distMap map[string]float64) BloomDistribution {
	return BloomDistribution{
		Remember:   distMap[constants.BloomLevelRemember],
		Understand: distMap[constants.BloomLevelUnderstand],
		Apply:      distMap[constants.BloomLevelApply],
		Analyze:    distMap[constants.BloomLevelAnalyze],
		Evaluate:   distMap[constants.BloomLevelEvaluate],
		Create:     distMap[constants.BloomLevelCreate],
	}
}

// Validate ensures the distribution totals to approximately 1.0
func (bd *BloomDistribution) Validate() error {
	total := bd.Remember + bd.Understand + bd.Apply + bd.Analyze + bd.Evaluate + bd.Create

	// Allow for small floating point errors
	if total < 0.95 || total > 1.05 {
		return fmt.Errorf("bloom distribution total is %.3f, expected ~1.0", total)
	}

	// Check for negative values
	values := []float64{bd.Remember, bd.Understand, bd.Apply, bd.Analyze, bd.Evaluate, bd.Create}
	for i, val := range values {
		if val < 0 {
			return fmt.Errorf("bloom level %s has negative value: %.3f", constants.BloomTaxonomyLevels[i], val)
		}
	}

	return nil
}

// Normalize adjusts the distribution to sum to 1.0
func (bd *BloomDistribution) Normalize() {
	total := bd.Remember + bd.Understand + bd.Apply + bd.Analyze + bd.Evaluate + bd.Create

	if total <= 0 {
		// If total is 0 or negative, set equal distribution
		equalValue := 1.0 / float64(len(constants.BloomTaxonomyLevels))
		bd.Remember = equalValue
		bd.Understand = equalValue
		bd.Apply = equalValue
		bd.Analyze = equalValue
		bd.Evaluate = equalValue
		bd.Create = equalValue
		return
	}

	// Normalize to sum to 1.0
	bd.Remember /= total
	bd.Understand /= total
	bd.Apply /= total
	bd.Analyze /= total
	bd.Evaluate /= total
	bd.Create /= total
}

// GetNonZeroLevels returns the Bloom levels that have non-zero distribution
func (bd *BloomDistribution) GetNonZeroLevels() []string {
	var levels []string
	values := map[string]float64{
		constants.BloomLevelRemember:   bd.Remember,
		constants.BloomLevelUnderstand: bd.Understand,
		constants.BloomLevelApply:      bd.Apply,
		constants.BloomLevelAnalyze:    bd.Analyze,
		constants.BloomLevelEvaluate:   bd.Evaluate,
		constants.BloomLevelCreate:     bd.Create,
	}

	for level, value := range values {
		if value > 0 {
			levels = append(levels, level)
		}
	}

	return levels
}

// DefaultBloomScoreConfig returns the default configuration with consolidated values from the existing codebase
func DefaultBloomScoreConfig() *BloomScoreConfig {
	return &BloomScoreConfig{
		BaseScores: map[string]int{
			constants.BloomLevelRemember:   10,
			constants.BloomLevelUnderstand: 15,
			constants.BloomLevelApply:      20,
			constants.BloomLevelAnalyze:    25,
			constants.BloomLevelEvaluate:   30,
			constants.BloomLevelCreate:     35,
		},
		StageMultipliers: map[string]float64{
			constants.DifficultyEasy:   1.0,
			constants.DifficultyMedium: 1.2,
			constants.DifficultyHard:   1.5,
		},
		InitialDistributions: map[string]BloomDistribution{
			constants.DifficultyEasy: {
				Remember:   0.5,
				Understand: 0.3,
				Apply:      0.2,
				Analyze:    0.0,
				Evaluate:   0.0,
				Create:     0.0,
			},
			constants.DifficultyMedium: {
				Remember:   0.0,
				Understand: 0.3,
				Apply:      0.4,
				Analyze:    0.3,
				Evaluate:   0.0,
				Create:     0.0,
			},
			constants.DifficultyHard: {
				Remember:   0.0,
				Understand: 0.0,
				Apply:      0.2,
				Analyze:    0.4,
				Evaluate:   0.3,
				Create:     0.1,
			},
		},
		RecoveryDistributions: map[string]BloomDistribution{
			constants.DifficultyEasy: {
				Remember:   0.6,
				Understand: 0.3,
				Apply:      0.1,
				Analyze:    0.0,
				Evaluate:   0.0,
				Create:     0.0,
			},
			constants.DifficultyMedium: {
				Remember:   0.4,
				Understand: 0.3,
				Apply:      0.2,
				Analyze:    0.1,
				Evaluate:   0.0,
				Create:     0.0,
			},
			constants.DifficultyHard: {
				Remember:   0.25,
				Understand: 0.25,
				Apply:      0.25,
				Analyze:    0.15,
				Evaluate:   0.1,
				Create:     0.0,
			},
		},
		RelaxedDistribution: BloomDistribution{
			Remember:   0.2,
			Understand: 0.2,
			Apply:      0.2,
			Analyze:    0.2,
			Evaluate:   0.1,
			Create:     0.1,
		},
	}
}

