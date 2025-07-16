package models

import "time"

type StageConfig struct {
	InitialQuestions  int     `bson:"initial_questions" json:"initial_questions"`
	PassingThreshold  float64 `bson:"passing_threshold" json:"passing_threshold"`
	RecoveryQuestions int     `bson:"recovery_questions" json:"recovery_questions"`
	RecoveryThreshold float64 `bson:"recovery_threshold" json:"recovery_threshold"`
	BasePoints        int     `bson:"base_points" json:"base_points"`
	RecoveryPoints    int     `bson:"recovery_points" json:"recovery_points"`
}

type Quiz struct {
	ID                   string                 `bson:"_id,omitempty" json:"id"`
	Title                string                 `bson:"title" json:"title"`
	Description          string                 `bson:"description" json:"description"`
	SkillID              string                 `bson:"skill_id" json:"skill_id"`
	TotalDurationSeconds int                    `bson:"total_duration_seconds" json:"total_duration_seconds"`
	MaxQuestions         int                    `bson:"max_questions" json:"max_questions"`
	StageConfig          map[string]StageConfig `bson:"stage_config" json:"stage_config"`
	Status               string                 `bson:"status" json:"status"`
	CreatedAt            time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time              `bson:"updated_at" json:"updated_at"`
}
