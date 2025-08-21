package event

import "time"

const (
	SessionResultEvent       = "quiz.result"
	SkillsQuizCompletedEvent = "skills.events.quiz_completed"
	SkillsProgressionEvent   = "skills.events.skill_progression"
	SkillsCognitiveEvent     = "skills.events.cognitive_profile"
	SkillsAnalyticsEvent     = "skills.events.learning_analytics"
)

// QuizCompletedEvent represents the consistent quiz completion event structure
type QuizCompletedEvent struct {
	// Core identifiers
	ResultID  string `json:"result_id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	QuizID    string `json:"quiz_id"`
	ConfigID  string `json:"config_id"`

	// Performance metrics
	FinalScore         float64 `json:"final_score"`
	Percentage         float64 `json:"percentage"`
	BadgeLevel         string  `json:"badge_level"`
	QuestionsAttempted int     `json:"questions_attempted"`
	QuestionsCorrect   int     `json:"questions_correct"`

	// Analysis data (keeping as interface{} to match existing patterns)
	BloomBreakdown interface{} `json:"bloom_breakdown"`
	StageBreakdown interface{} `json:"stage_breakdown"`
	TimeBreakdown  interface{} `json:"time_breakdown"`
	CompletionType string      `json:"completion_type"`

	// Enhanced analytics (optional fields)
	SkillProgressions  []interface{} `json:"skill_progressions,omitempty"`
	CognitiveProfile   interface{}   `json:"cognitive_profile,omitempty"`
	LearningPatterns   interface{}   `json:"learning_patterns,omitempty"`
	PerformanceMetrics interface{}   `json:"performance_metrics,omitempty"`
	SessionMetadata    interface{}   `json:"session_metadata,omitempty"`

	// Event metadata
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Source    string    `json:"source"`
	CreatedAt string    `json:"created_at"`
}
