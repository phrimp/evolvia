package models

import "time"

// QuizAnswer represents an answer submitted for a question in a quiz session
//
// DEPRECATED PERSISTENCE: As of the cache-based implementation, individual answers are no longer
// persisted to the database. Instead, they are cached in memory through SessionService.
// This model is still used for:
// - API compatibility and response formatting
// - Converting between cached answers and API responses
// - Backward compatibility with existing integrations
//
// For current implementations:
// - Use SessionService.CacheAnswer() to store answers in memory
// - Use SessionService.GetCachedAnswers() to retrieve session answers
// - Cache provides better performance and includes enhanced timing metadata
// - See CachedAnswer model for the enhanced in-memory representation
type QuizAnswer struct {
	ID               string    `bson:"_id,omitempty" json:"id"`
	SessionID        string    `bson:"session_id" json:"session_id"`
	QuestionID       string    `bson:"question_id" json:"question_id"`
	UserAnswer       string    `bson:"user_answer" json:"user_answer"`
	IsCorrect        bool      `bson:"is_correct" json:"is_correct"`
	PointsEarned     float64   `bson:"points_earned" json:"points_earned"`
	TimeSpentSeconds int       `bson:"time_spent_seconds" json:"time_spent_seconds"`
	AnsweredAt       time.Time `bson:"answered_at" json:"answered_at"`
	StageType        string    `bson:"stage_type" json:"stage_type"`
	QuestionSequence int       `bson:"question_sequence" json:"question_sequence"`
}
