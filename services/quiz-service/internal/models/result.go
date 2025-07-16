package models

import "time"

type StageBreakdown struct {
	Attempted    int     `bson:"attempted" json:"attempted"`
	Correct      int     `bson:"correct" json:"correct"`
	Score        float64 `bson:"score" json:"score"`
	Percentage   float64 `bson:"percentage" json:"percentage"`
	Passed       bool    `bson:"passed" json:"passed"`
	RecoveryUsed bool    `bson:"recovery_used" json:"recovery_used"`
}

type TimeBreakdown struct {
	TotalTimeSeconds       int            `bson:"total_time_seconds" json:"total_time_seconds"`
	AverageTimePerQuestion float64        `bson:"average_time_per_question" json:"average_time_per_question"`
	TimeByStage            map[string]int `bson:"time_by_stage" json:"time_by_stage"`
}

type BloomBreakdown struct {
	Remember   map[string]int `bson:"remember" json:"remember"`
	Understand map[string]int `bson:"understand" json:"understand"`
	Apply      map[string]int `bson:"apply" json:"apply"`
	Analyze    map[string]int `bson:"analyze" json:"analyze"`
	Evaluate   map[string]int `bson:"evaluate" json:"evaluate"`
	Create     map[string]int `bson:"create" json:"create"`
}

type QuizResult struct {
	ID                 string                    `bson:"_id,omitempty" json:"id"`
	SessionID          string                    `bson:"session_id" json:"session_id"`
	UserID             string                    `bson:"user_id" json:"user_id"`
	QuizID             string                    `bson:"quiz_id" json:"quiz_id"`
	FinalScore         float64                   `bson:"final_score" json:"final_score"`
	Percentage         float64                   `bson:"percentage" json:"percentage"`
	BadgeLevel         string                    `bson:"badge_level" json:"badge_level"`
	QuestionsAttempted int                       `bson:"questions_attempted" json:"questions_attempted"`
	QuestionsCorrect   int                       `bson:"questions_correct" json:"questions_correct"`
	StageBreakdown     map[string]StageBreakdown `bson:"stage_breakdown" json:"stage_breakdown"`
	TimeBreakdown      TimeBreakdown             `bson:"time_breakdown" json:"time_breakdown"`
	BloomBreakdown     BloomBreakdown            `bson:"bloom_breakdown" json:"bloom_breakdown"`
	CompletionType     string                    `bson:"completion_type" json:"completion_type"`
	CreatedAt          time.Time                 `bson:"created_at" json:"created_at"`
}
