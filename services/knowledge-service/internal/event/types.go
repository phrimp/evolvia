package event

import (
	"knowledge-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// Skill events
	EventTypeSkillCreated     = "skill.created"
	EventTypeSkillUpdated     = "skill.updated"
	EventTypeSkillDeleted     = "skill.deleted"
	EventTypeSkillActivated   = "skill.activated"
	EventTypeSkillDeactivated = "skill.deactivated"

	// User skill events
	EventTypeUserSkillAdded    = "user_skill.added"
	EventTypeUserSkillUpdated  = "user_skill.updated"
	EventTypeUserSkillRemoved  = "user_skill.removed"
	EventTypeUserSkillEndorsed = "user_skill.endorsed"
	EventTypeUserSkillVerified = "user_skill.verified"
	EventTypeUserSkillUsed     = "user_skill.used"

	// Skill extraction events
	EventTypeSkillsExtracted = "skills.extracted"
	EventTypeSkillMatched    = "skill.matched"

	// Quiz result events
	EventTypeQuizResultCompleted = "quiz.result.completed"

	// System events
	EventTypeDataReloaded = "data.reloaded"
)

// ConsistentQuizCompletedEvent matches the quiz service event structure for compatibility
type ConsistentQuizCompletedEvent struct {
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

	// Analysis data
	BloomBreakdown interface{} `json:"bloom_breakdown"`
	StageBreakdown interface{} `json:"stage_breakdown"`
	TimeBreakdown  interface{} `json:"time_breakdown"`
	CompletionType string      `json:"completion_type"`

	// Enhanced analytics (optional)
	SkillProgressions []interface{} `json:"skill_progressions,omitempty"`
	CognitiveProfile  interface{}   `json:"cognitive_profile,omitempty"`
	LearningPatterns  interface{}   `json:"learning_patterns,omitempty"`

	// Event metadata
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Source    string    `json:"source"`
	CreatedAt string    `json:"created_at"`
}

// SkillEvent represents skill-related events
type SkillEvent struct {
	EventType     string         `json:"eventType"`
	SkillID       string         `json:"skillId"`
	SkillName     string         `json:"skillName"`
	CategoryID    string         `json:"categoryId,omitempty"`
	Timestamp     int64          `json:"timestamp"`
	ChangedFields []string       `json:"changedFields,omitempty"`
	OldValues     map[string]any `json:"oldValues,omitempty"`
	NewValues     map[string]any `json:"newValues,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// UserSkillEvent represents user skill-related events
type UserSkillEvent struct {
	EventType       string            `json:"eventType"`
	UserSkillID     string            `json:"userSkillId"`
	UserID          string            `json:"userId"`
	SkillID         string            `json:"skillId"`
	SkillName       string            `json:"skillName"`
	Level           models.SkillLevel `json:"level"`
	Confidence      float64           `json:"confidence"`
	YearsExperience int               `json:"yearsExperience"`
	Verified        bool              `json:"verified"`
	Endorsements    int               `json:"endorsements"`
	Timestamp       int64             `json:"timestamp"`
	ChangedFields   []string          `json:"changedFields,omitempty"`
	OldValues       map[string]any    `json:"oldValues,omitempty"`
	NewValues       map[string]any    `json:"newValues,omitempty"`
	Source          string            `json:"source,omitempty"` // manual, extracted, imported
}

// SkillExtractionEvent represents skill extraction from text
type SkillExtractionEvent struct {
	EventType      string            `json:"eventType"`
	UserID         string            `json:"userId"`
	SourceType     string            `json:"sourceType"` // resume, profile, job_description, etc.
	SourceID       string            `json:"sourceId"`
	ExtractedText  string            `json:"extractedText"`
	SkillMatches   []SkillMatchEvent `json:"skillMatches"`
	ProcessingTime time.Duration     `json:"processingTime"`
	Timestamp      int64             `json:"timestamp"`
}

// SkillMatchEvent represents a skill identified in text
type SkillMatchEvent struct {
	SkillID        string            `json:"skillId"`
	SkillName      string            `json:"skillName"`
	MatchedText    string            `json:"matchedText"`
	Confidence     float64           `json:"confidence"`
	Context        string            `json:"context"`
	Position       int               `json:"position"`
	SuggestedLevel models.SkillLevel `json:"suggestedLevel"`
}

// InputSkillEvent represents incoming skill data for processing
type InputSkillEvent struct {
	EventType   string              `json:"event_type"`
	UserID      string              `json:"user_id"`
	UserEmail   string              `json:"user_email,omitempty"`
	Source      string              `json:"source"` // resume, linkedin, manual, etc.
	SourceID    string              `json:"source_id,omitempty"`
	Timestamp   string              `json:"timestamp"`
	ProcessMode string              `json:"process_mode,omitempty"` // auto, manual, review
	Data        InputSkillEventData `json:"data"`
}

// InputSkillEventData represents the data payload of input skill event
type InputSkillEventData struct {
	Filename           string                 `json:"filename"`
	ContentType        string                 `json:"content_type"`
	ExtractedContent   map[string]interface{} `json:"extracted_content"`
	FileBinary         string                 `json:"file_binary"`
	TextForAnalysis    string                 `json:"text_for_analysis"`
	ProcessingMetadata map[string]interface{} `json:"processing_metadata"`
}

// QuizResultEvent represents quiz completion result events
type QuizResultEvent struct {
	ResultID             string                    `json:"result_id"`
	SessionID            string                    `json:"session_id"`
	UserID               string                    `json:"user_id"`
	QuizID               string                    `json:"quiz_id"`
	ConfigID             string                    `json:"config_id"`
	FinalScore           float64                   `json:"final_score"`
	Percentage           float64                   `json:"percentage"`
	BadgeLevel           string                    `json:"badge_level"`
	QuestionsAttempted   int                       `json:"questions_attempted"`
	QuestionsCorrect     int                       `json:"questions_correct"`
	BloomBreakdown       QuizBloomBreakdown        `json:"bloom_breakdown"`
	StageBreakdown       map[string]StageBreakdown `json:"stage_breakdown"`
	TimeBreakdown        TimeBreakdown             `json:"time_breakdown"`
	CompletionType       string                    `json:"completion_type"`
	CreatedAt            string                    `json:"created_at"`
	
	// Enhanced data fields
	SkillProgressions    []SkillProgression        `json:"skill_progressions"`
	CognitiveProfile     CognitiveProfile          `json:"cognitive_profile"`
	LearningPatterns     LearningPatterns          `json:"learning_patterns"`
	PerformanceMetrics   PerformanceMetrics        `json:"performance_metrics"`
	SessionMetadata      SessionMetadata           `json:"session_metadata"`
	
	// Additional context
	Timestamp            time.Time                 `json:"timestamp"`
	EventType            string                    `json:"event_type"`
	Exchange             string                    `json:"exchange"`
}

// QuizBloomBreakdown represents Bloom's taxonomy breakdown from quiz
type QuizBloomBreakdown struct {
	Remember   QuizBloomLevelPerformance `json:"remember"`
	Understand QuizBloomLevelPerformance `json:"understand"`
	Apply      QuizBloomLevelPerformance `json:"apply"`
	Analyze    QuizBloomLevelPerformance `json:"analyze"`
	Evaluate   QuizBloomLevelPerformance `json:"evaluate"`
	Create     QuizBloomLevelPerformance `json:"create"`
	Summary    CognitiveProfile          `json:"summary"`
}

// QuizBloomLevelPerformance represents performance at a specific Bloom's level
type QuizBloomLevelPerformance struct {
	QuestionsAttempted   int     `json:"questions_attempted"`
	QuestionsCorrect     int     `json:"questions_correct"`
	ActualScore          float64 `json:"actual_score"`
	PossibleScore        float64 `json:"possible_score"`
	AccuracyPercentage   float64 `json:"accuracy_percentage"`
	ScorePercentage      float64 `json:"score_percentage"`
	AverageQuestionScore float64 `json:"avg_question_score"`
	EfficiencyRating     string  `json:"efficiency_rating"`
	TotalTimeSpent       int     `json:"total_time_spent"`
	AverageTimePerQ      float64 `json:"avg_time_per_question"`
}

// StageBreakdown represents performance breakdown by quiz stage
type StageBreakdown struct {
	Attempted    int     `json:"attempted"`
	Correct      int     `json:"correct"`
	Score        float64 `json:"score"`
	Percentage   float64 `json:"percentage"`
	Passed       bool    `json:"passed"`
	RecoveryUsed bool    `json:"recovery_used"`
}

// TimeBreakdown represents time analysis of quiz performance
type TimeBreakdown struct {
	TotalTimeSeconds       int            `json:"total_time_seconds"`
	AverageTimePerQuestion float64        `json:"average_time_per_question"`
	TimeByStage            map[string]int `json:"time_by_stage"`
}

// CognitiveProfile represents cognitive analysis summary
type CognitiveProfile struct {
	DominantStrengths       []string `json:"dominant_strengths"`
	GrowthAreas             []string `json:"growth_areas"`
	CognitiveComplexity     float64  `json:"cognitive_complexity"`
	OverallPercentage       float64  `json:"overall_percentage"`
	LearningRecommendations []string `json:"learning_recommendations"`
}

// SkillProgression represents progress tracking for specific skills
type SkillProgression struct {
	SkillID           string                        `json:"skill_id"`
	SkillName         string                        `json:"skill_name"`
	PreQuizLevel      string                        `json:"pre_quiz_level"`
	PostQuizLevel     string                        `json:"post_quiz_level"`
	ProgressGain      float64                       `json:"progress_gain"`
	BloomImprovement  models.BloomsTaxonomyAssessment `json:"bloom_improvement"`
	ConfidenceChange  float64                       `json:"confidence_change"`
	Verified          bool                          `json:"verified"`
}

// LearningPatterns represents user learning behavior analysis
type LearningPatterns struct {
	PreferredBloomLevels  []string             `json:"preferred_bloom_levels"`
	LearningStyle         string               `json:"learning_style"`
	ResponsePatterns      map[string]float64   `json:"response_patterns"`
	TimeManagement        TimeManagementProfile `json:"time_management"`
	EngagementMetrics     EngagementMetrics    `json:"engagement_metrics"`
	AdaptabilityScore     float64              `json:"adaptability_score"`
}

// TimeManagementProfile represents how user manages time during quiz
type TimeManagementProfile struct {
	AverageThinkTime      float64 `json:"average_think_time"`
	RushingTendency       float64 `json:"rushing_tendency"`
	PauseBehavior         float64 `json:"pause_behavior"`
	ConsistencyScore      float64 `json:"consistency_score"`
}

// EngagementMetrics represents user engagement during quiz
type EngagementMetrics struct {
	AttentionScore        float64 `json:"attention_score"`
	PersistenceScore      float64 `json:"persistence_score"`
	FocusConsistency      float64 `json:"focus_consistency"`
	InteractionQuality    float64 `json:"interaction_quality"`
}

// PerformanceMetrics represents comprehensive performance analysis
type PerformanceMetrics struct {
	FinalScore           float64             `json:"final_score"`
	BadgeLevel           string              `json:"badge_level"`
	StageBreakdown       map[string]StageBreakdown `json:"stage_breakdown"`
	BloomBreakdown       QuizBloomBreakdown  `json:"bloom_breakdown"`
	TimeBreakdown        TimeBreakdown       `json:"time_breakdown"`
	CompletionType       string              `json:"completion_type"`
	DurationSeconds      int                 `json:"duration_seconds"`
	QuestionsAttempted   int                 `json:"questions_attempted"`
	QuestionsCorrect     int                 `json:"questions_correct"`
	EfficiencyRating     string              `json:"efficiency_rating"`
	DifficultyProgression float64            `json:"difficulty_progression"`
	AccuracyTrend        []float64           `json:"accuracy_trend"`
}

// SessionMetadata represents metadata about the quiz session
type SessionMetadata struct {
	SkillID            string               `json:"skill_id"`
	TotalQuestions     int                  `json:"total_questions"`
	StagesCompleted    int                  `json:"stages_completed"`
	RecoveryRounds     int                  `json:"recovery_rounds"`
	AdaptiveAdjustments int                 `json:"adaptive_adjustments"`
	DeviceType         string               `json:"device_type"`
	ConnectionQuality  string               `json:"connection_quality"`
	StartTime          time.Time            `json:"start_time"`
	EndTime            time.Time            `json:"end_time"`
}

// InputSkill represents a single skill input
type InputSkill struct {
	Name            string            `json:"name"`
	Level           models.SkillLevel `json:"level,omitempty"`
	YearsExperience int               `json:"yearsExperience,omitempty"`
	Confidence      float64           `json:"confidence,omitempty"`
	Context         string            `json:"context,omitempty"`
	Verified        bool              `json:"verified,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
}

// SystemEvent represents system-level events
type SystemEvent struct {
	EventType string         `json:"eventType"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Timestamp int64          `json:"timestamp"`
}

// Event factory functions

// CreateSkillCreatedEvent creates a skill created event
func CreateSkillCreatedEvent(skill *models.Skill) *SkillEvent {
	var categoryID string
	if skill.CategoryID != nil {
		categoryID = skill.CategoryID.Hex()
	}

	return &SkillEvent{
		EventType:  EventTypeSkillCreated,
		SkillID:    skill.ID.Hex(),
		SkillName:  skill.Name,
		CategoryID: categoryID,
		Timestamp:  time.Now().Unix(),
	}
}

// CreateSkillUpdatedEvent creates a skill updated event
func CreateSkillUpdatedEvent(skill *models.Skill, changedFields []string, oldValues, newValues map[string]any) *SkillEvent {
	var categoryID string
	if skill.CategoryID != nil {
		categoryID = skill.CategoryID.Hex()
	}

	return &SkillEvent{
		EventType:     EventTypeSkillUpdated,
		SkillID:       skill.ID.Hex(),
		SkillName:     skill.Name,
		CategoryID:    categoryID,
		Timestamp:     time.Now().Unix(),
		ChangedFields: changedFields,
		OldValues:     oldValues,
		NewValues:     newValues,
	}
}

// CreateSkillDeletedEvent creates a skill deleted event
func CreateSkillDeletedEvent(skill *models.Skill) *SkillEvent {
	var categoryID string
	if skill.CategoryID != nil {
		categoryID = skill.CategoryID.Hex()
	}

	return &SkillEvent{
		EventType:  EventTypeSkillDeleted,
		SkillID:    skill.ID.Hex(),
		SkillName:  skill.Name,
		CategoryID: categoryID,
		Timestamp:  time.Now().Unix(),
	}
}

// CreateUserSkillAddedEvent creates a user skill added event
func CreateUserSkillAddedEvent(userSkill *models.UserSkill, skillName, source string) *UserSkillEvent {
	return &UserSkillEvent{
		EventType:       EventTypeUserSkillAdded,
		UserSkillID:     userSkill.ID.Hex(),
		UserID:          userSkill.UserID.Hex(),
		SkillID:         userSkill.SkillID.Hex(),
		SkillName:       skillName,
		Level:           userSkill.Level,
		Confidence:      userSkill.Confidence,
		YearsExperience: userSkill.YearsExperience,
		Verified:        userSkill.Verified,
		Endorsements:    userSkill.Endorsements,
		Timestamp:       time.Now().Unix(),
		Source:          source,
	}
}

// CreateUserSkillUpdatedEvent creates a user skill updated event
func CreateUserSkillUpdatedEvent(userSkill *models.UserSkill, skillName string, changedFields []string, oldValues, newValues map[string]any) *UserSkillEvent {
	return &UserSkillEvent{
		EventType:       EventTypeUserSkillUpdated,
		UserSkillID:     userSkill.ID.Hex(),
		UserID:          userSkill.UserID.Hex(),
		SkillID:         userSkill.SkillID.Hex(),
		SkillName:       skillName,
		Level:           userSkill.Level,
		Confidence:      userSkill.Confidence,
		YearsExperience: userSkill.YearsExperience,
		Verified:        userSkill.Verified,
		Endorsements:    userSkill.Endorsements,
		Timestamp:       time.Now().Unix(),
		ChangedFields:   changedFields,
		OldValues:       oldValues,
		NewValues:       newValues,
	}
}

// CreateUserSkillRemovedEvent creates a user skill removed event
func CreateUserSkillRemovedEvent(userID, skillID bson.ObjectID, skillName string) *UserSkillEvent {
	return &UserSkillEvent{
		EventType: EventTypeUserSkillRemoved,
		UserID:    userID.Hex(),
		SkillID:   skillID.Hex(),
		SkillName: skillName,
		Timestamp: time.Now().Unix(),
	}
}

// CreateUserSkillEndorsedEvent creates a user skill endorsed event
func CreateUserSkillEndorsedEvent(userSkill *models.UserSkill, skillName string) *UserSkillEvent {
	return &UserSkillEvent{
		EventType:    EventTypeUserSkillEndorsed,
		UserSkillID:  userSkill.ID.Hex(),
		UserID:       userSkill.UserID.Hex(),
		SkillID:      userSkill.SkillID.Hex(),
		SkillName:    skillName,
		Endorsements: userSkill.Endorsements,
		Timestamp:    time.Now().Unix(),
	}
}

// CreateUserSkillVerifiedEvent creates a user skill verified event
func CreateUserSkillVerifiedEvent(userSkill *models.UserSkill, skillName string, verified bool) *UserSkillEvent {
	return &UserSkillEvent{
		EventType:   EventTypeUserSkillVerified,
		UserSkillID: userSkill.ID.Hex(),
		UserID:      userSkill.UserID.Hex(),
		SkillID:     userSkill.SkillID.Hex(),
		SkillName:   skillName,
		Verified:    verified,
		Timestamp:   time.Now().Unix(),
		OldValues:   map[string]any{"verified": !verified},
		NewValues:   map[string]any{"verified": verified},
	}
}

// CreateUserSkillUsedEvent creates a user skill used event
func CreateUserSkillUsedEvent(userSkill *models.UserSkill, skillName string) *UserSkillEvent {
	return &UserSkillEvent{
		EventType:   EventTypeUserSkillUsed,
		UserSkillID: userSkill.ID.Hex(),
		UserID:      userSkill.UserID.Hex(),
		SkillID:     userSkill.SkillID.Hex(),
		SkillName:   skillName,
		Timestamp:   time.Now().Unix(),
	}
}

// CreateSkillsExtractedEvent creates a skills extracted event
func CreateSkillsExtractedEvent(userID, sourceID, sourceType string, matches []SkillMatchEvent, processingTime time.Duration) *SkillExtractionEvent {
	return &SkillExtractionEvent{
		EventType:      EventTypeSkillsExtracted,
		UserID:         userID,
		SourceType:     sourceType,
		SourceID:       sourceID,
		SkillMatches:   matches,
		ProcessingTime: processingTime,
		Timestamp:      time.Now().Unix(),
	}
}

// CreateDataReloadedEvent creates a data reloaded event
func CreateDataReloadedEvent(details map[string]any) *SystemEvent {
	return &SystemEvent{
		EventType: EventTypeDataReloaded,
		Message:   "Skill data reloaded successfully",
		Details:   details,
		Timestamp: time.Now().Unix(),
	}
}
