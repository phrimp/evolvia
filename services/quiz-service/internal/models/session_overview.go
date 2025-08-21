package models

import "time"

// UserSessionOverview represents the overview of all sessions for a user
type UserSessionOverview struct {
	UserID     string           `json:"user_id"`
	Sessions   []SessionSummary `json:"sessions"`
	Pagination Pagination       `json:"pagination"`
}

// SessionSummary represents a condensed view of session data for overview
type SessionSummary struct {
	ID              string                 `json:"id"`
	ConfigID        string                 `json:"config_id"`
	Status          string                 `json:"status"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         time.Time              `json:"end_time,omitempty"`
	CurrentStage    string                 `json:"current_stage"`
	TotalQuestions  int                    `json:"total_questions"`
	FinalScore      float64                `json:"final_score"`
	SkillInfo       map[string]interface{} `json:"skill_info,omitempty"`
	ProgressSummary ProgressSummary        `json:"progress_summary"`
}

// SessionDetails represents detailed session information with cached questions
type SessionDetails struct {
	Session         *QuizSession           `json:"session"`
	CachedQuestions []CachedAnswer         `json:"cached_questions"`
	BloomBreakdown  BloomBreakdown         `json:"bloom_breakdown"`
	CacheInfo       map[string]interface{} `json:"cache_info"`
}

// ProgressSummary provides quick progress metrics for session overview
type ProgressSummary struct {
	QuestionsAnswered int                             `json:"questions_answered"`
	OverallProgress   float64                         `json:"overall_progress"`
	StageBreakdown    map[string]StageProgressSummary `json:"stage_breakdown"`
}

// StageProgressSummary provides condensed stage progress information
type StageProgressSummary struct {
	Passed   bool    `json:"passed"`
	Accuracy float64 `json:"accuracy"`
}

// Pagination represents pagination information
type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

