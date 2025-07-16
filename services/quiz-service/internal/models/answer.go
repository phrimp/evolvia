package models

import "time"

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
