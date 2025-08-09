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

type BloomLevelPerformance struct {
	QuestionsAttempted   int     `bson:"questions_attempted" json:"questions_attempted"`
	QuestionsCorrect     int     `bson:"questions_correct" json:"questions_correct"`
	ActualScore          float64 `bson:"actual_score" json:"actual_score"`
	PossibleScore        float64 `bson:"possible_score" json:"possible_score"`
	AccuracyPercentage   float64 `bson:"accuracy_percentage" json:"accuracy_percentage"`
	ScorePercentage      float64 `bson:"score_percentage" json:"score_percentage"`
	AverageQuestionScore float64 `bson:"avg_question_score" json:"avg_question_score"`
	EfficiencyRating     string  `bson:"efficiency_rating" json:"efficiency_rating"`
	TotalTimeSpent       int     `bson:"total_time_spent" json:"total_time_spent"`
	AverageTimePerQ      float64 `bson:"avg_time_per_question" json:"avg_time_per_question"`
}

type CognitiveProfile struct {
	DominantStrengths       []string `bson:"dominant_strengths" json:"dominant_strengths"`
	GrowthAreas             []string `bson:"growth_areas" json:"growth_areas"`
	CognitiveComplexity     float64  `bson:"cognitive_complexity" json:"cognitive_complexity"`
	OverallPercentage       float64  `bson:"overall_percentage" json:"overall_percentage"`
	LearningRecommendations []string `bson:"learning_recommendations" json:"learning_recommendations"`
}

type BloomBreakdown struct {
	Remember   BloomLevelPerformance `bson:"remember" json:"remember"`
	Understand BloomLevelPerformance `bson:"understand" json:"understand"`
	Apply      BloomLevelPerformance `bson:"apply" json:"apply"`
	Analyze    BloomLevelPerformance `bson:"analyze" json:"analyze"`
	Evaluate   BloomLevelPerformance `bson:"evaluate" json:"evaluate"`
	Create     BloomLevelPerformance `bson:"create" json:"create"`
	Summary    CognitiveProfile      `bson:"summary" json:"summary"`
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
