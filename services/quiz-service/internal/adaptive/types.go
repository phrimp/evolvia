package adaptive

type Stage string

const (
	StageEasy   Stage = "easy"
	StageMedium Stage = "medium"
	StageHard   Stage = "hard"
)

// StageStatus tracks the current progress within a stage
type StageStatus struct {
	Stage          Stage   `json:"stage"`
	QuestionsAsked int     `json:"questions_asked"`
	CorrectAnswers int     `json:"correct_answers"`
	InRecovery     bool    `json:"in_recovery"`
	RecoveryRound  int     `json:"recovery_round"`
	Passed         bool    `json:"passed"`
	Score          float64 `json:"score"`
}

// AdaptiveSession represents the adaptive quiz session state
type AdaptiveSession struct {
	SessionID           string                 `json:"session_id"`
	CurrentStage        Stage                  `json:"current_stage"`
	StageStatuses       map[Stage]*StageStatus `json:"stage_statuses"`
	TotalQuestionsAsked int                    `json:"total_questions_asked"`
	UsedQuestionIDs     []string               `json:"used_question_ids"`
	IsComplete          bool                   `json:"is_complete"`
	TotalScore          float64                `json:"total_score"`
}

// AdaptiveConfig holds the configuration for adaptive quiz behavior
type AdaptiveConfig struct {
	MaxQuestions int                   `json:"max_questions"`
	StageConfigs map[Stage]StageConfig `json:"stage_configs"`
}

// StageConfig defines behavior for each stage
type StageConfig struct {
	InitialQuestions  int     `json:"initial_questions"`
	PassingThreshold  float64 `json:"passing_threshold"`
	RecoveryQuestions int     `json:"recovery_questions"`
	RecoveryThreshold float64 `json:"recovery_threshold"`
	BasePoints        float64 `json:"base_points"`
	RecoveryPoints    float64 `json:"recovery_points"`
}

// QuestionRequest represents a request for next question
type QuestionRequest struct {
	SessionID  string   `json:"session_id"`
	SkillID    string   `json:"skill_id"`
	Stage      Stage    `json:"stage"`
	ExcludeIDs []string `json:"exclude_ids"`
	IsRecovery bool     `json:"is_recovery"`
}

// AnswerResult represents the result of answering a question
type AnswerResult struct {
	IsCorrect    bool    `json:"is_correct"`
	PointsEarned float64 `json:"points_earned"`
	StageUpdate  bool    `json:"stage_update"`
	NextStage    Stage   `json:"next_stage,omitempty"`
	IsComplete   bool    `json:"is_complete"`
}

// Default configuration based on requirements
func DefaultAdaptiveConfig() *AdaptiveConfig {
	return &AdaptiveConfig{
		MaxQuestions: 25,
		StageConfigs: map[Stage]StageConfig{
			StageEasy: {
				InitialQuestions:  5,
				PassingThreshold:  0.8, // 4/5 correct
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.67, // 2/3 correct
				BasePoints:        3,
				RecoveryPoints:    2,
			},
			StageMedium: {
				InitialQuestions:  5,
				PassingThreshold:  0.8, // 4/5 correct
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.67, // 2/3 correct
				BasePoints:        7,
				RecoveryPoints:    4,
			},
			StageHard: {
				InitialQuestions:  5,
				PassingThreshold:  0.6, // 3/5 correct
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.67, // 2/3 correct
				BasePoints:        10,
				RecoveryPoints:    6,
			},
		},
	}
}

// NewAdaptiveSession creates a new adaptive session
func NewAdaptiveSession(sessionID string) *AdaptiveSession {
	return &AdaptiveSession{
		SessionID:    sessionID,
		CurrentStage: StageEasy,
		StageStatuses: map[Stage]*StageStatus{
			StageEasy: {
				Stage: StageEasy,
			},
			StageMedium: {
				Stage: StageMedium,
			},
			StageHard: {
				Stage: StageHard,
			},
		},
		UsedQuestionIDs: []string{},
		IsComplete:      false,
		TotalScore:      0,
	}
}
