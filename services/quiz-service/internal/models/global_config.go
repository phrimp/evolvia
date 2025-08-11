package models

import (
	"fmt"
	"time"
)

// GlobalQuizConfig defines global configuration for all quiz sessions
// Replaces the Quiz model to eliminate quiz-specific question pools
type GlobalQuizConfig struct {
	ID                   string                 `bson:"_id,omitempty" json:"id"`
	Name                 string                 `bson:"name" json:"name"`
	Description          string                 `bson:"description" json:"description"`
	StageConfig          map[string]StageConfig `bson:"stage_config" json:"stage_config"`
	TotalDurationSeconds int                    `bson:"total_duration_seconds" json:"total_duration_seconds"`
	MaxQuestions         int                    `bson:"max_questions" json:"max_questions"`
	IsDefault            bool                   `bson:"is_default" json:"is_default"`
	Status               string                 `bson:"status" json:"status"`
	CreatedAt            time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time              `bson:"updated_at" json:"updated_at"`
}

// DefaultGlobalConfig creates a default configuration
func DefaultGlobalConfig() *GlobalQuizConfig {
	return &GlobalQuizConfig{
		Name:        "Default Adaptive Configuration",
		Description: "Default configuration for adaptive quiz sessions",
		StageConfig: map[string]StageConfig{
			"easy": {
				InitialQuestions:  5,
				PassingThreshold:  0.6,
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.5,
				BasePoints:        10,
				RecoveryPoints:    5,
			},
			"medium": {
				InitialQuestions:  5,
				PassingThreshold:  0.7,
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.6,
				BasePoints:        15,
				RecoveryPoints:    8,
			},
			"hard": {
				InitialQuestions:  5,
				PassingThreshold:  0.8,
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.7,
				BasePoints:        20,
				RecoveryPoints:    12,
			},
		},
		TotalDurationSeconds: 3600, // 60 minutes
		MaxQuestions:         20,
		IsDefault:            true,
		Status:               "active",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
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
