package models

import "time"

type StageProgress struct {
	Attempted     int     `bson:"attempted" json:"attempted"`
	Correct       int     `bson:"correct" json:"correct"`
	Passed        bool    `bson:"passed" json:"passed"`
	RecoveryRound int     `bson:"recovery_round" json:"recovery_round"`
	Score         float64 `bson:"score" json:"score"`
}

type QuizSession struct {
	ID                  string                   `bson:"_id,omitempty" json:"id"`
	QuizID              string                   `bson:"quiz_id" json:"quiz_id"`
	UserID              string                   `bson:"user_id" json:"user_id"`
	SessionToken        string                   `bson:"session_token" json:"session_token"`
	StartTime           time.Time                `bson:"start_time" json:"start_time"`
	EndTime             time.Time                `bson:"end_time" json:"end_time"`
	DurationSeconds     int                      `bson:"duration_seconds" json:"duration_seconds"`
	CurrentStage        string                   `bson:"current_stage" json:"current_stage"`
	TotalQuestionsAsked int                      `bson:"total_questions_asked" json:"total_questions_asked"`
	StageProgress       map[string]StageProgress `bson:"stage_progress" json:"stage_progress"`
	QuestionsUsed       []string                 `bson:"questions_used" json:"questions_used"`
	Status              string                   `bson:"status" json:"status"`
	FinalScore          float64                  `bson:"final_score" json:"final_score"`
	CompletionType      string                   `bson:"completion_type" json:"completion_type"`
}
