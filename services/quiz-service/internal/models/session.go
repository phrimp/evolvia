// services/quiz-service/internal/models/session.go
package models

import "time"

// Session Architecture Evolution:
// DEPRECATED: Quiz-dependent sessions (using QuizID) - legacy implementation
// CURRENT: Global sessions (using ConfigID) - recommended for all new implementations
// 
// Benefits of Global Sessions:
// - No quiz dependency constraints
// - Better performance with global configurations  
// - Enhanced caching capabilities
// - Simplified session management
// - Forward compatibility with configuration-based architecture

type StageProgress struct {
	Attempted     int     `bson:"attempted" json:"attempted"`
	Correct       int     `bson:"correct" json:"correct"`
	Passed        bool    `bson:"passed" json:"passed"`
	RecoveryRound int     `bson:"recovery_round" json:"recovery_round"`
	Score         float64 `bson:"score" json:"score"`
}

type QuizSession struct {
	ID                  string                   `bson:"_id,omitempty" json:"id"`
	ConfigID            string                   `bson:"config_id" json:"config_id"` // Changed from QuizID to ConfigID
	UserID              string                   `bson:"user_id" json:"user_id"`
	SessionToken        string                   `bson:"session_token" json:"session_token"`
	StartTime           time.Time                `bson:"start_time" json:"start_time"`
	EndTime             time.Time                `bson:"end_time" json:"end_time"`
	DurationSeconds     int                      `bson:"duration_seconds" json:"duration_seconds"`
	CurrentStage        string                   `bson:"current_stage" json:"current_stage"`
	TotalQuestionsAsked int                      `bson:"total_questions_asked" json:"total_questions_asked"`
	StageProgress       map[string]StageProgress `bson:"stage_progress" json:"stage_progress"`
	QuestionsUsed       []string                 `bson:"questions_used" json:"questions_used"`
	AnsweredQuestionIDs []string                 `bson:"answered_question_ids" json:"answered_question_ids"`
	Status              string                   `bson:"status" json:"status"`
	FinalScore          float64                  `bson:"final_score" json:"final_score"`
	CompletionType      string                   `bson:"completion_type" json:"completion_type"`

	// New field to store skill information and other metadata
	Metadata map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// SessionMetadata structure for type-safe access to metadata
type SessionMetadata struct {
	SkillID        string              `json:"skill_id"`
	SkillName      string              `json:"skill_name"`
	SkillTags      []string            `json:"skill_tags"`
	QuestionPools  map[string][]string `json:"question_pools,omitempty"`
	QuizStartTime  int64               `json:"quiz_start_time"`
	AdaptiveConfig map[string]any      `json:"adaptive_config,omitempty"`
}
