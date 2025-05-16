package models

type LearningPreferences struct {
	PreferredPace       LearningPace             `json:"preferredPace" bson:"preferredPace"`
	PreferredDifficulty DifficultyLevel          `json:"preferredDifficulty" bson:"preferredDifficulty"`
	PreferredFormats    []ContentFormat          `json:"preferredFormats" bson:"preferredFormats"`
	DailyGoalMinutes    int                      `json:"dailyGoalMinutes" bson:"dailyGoalMinutes"`
	WeeklyGoalMinutes   int                      `json:"weeklyGoalMinutes" bson:"weeklyGoalMinutes"`
	PreferredTime       []string                 `json:"preferredTime,omitempty" bson:"preferredTime,omitempty"`
	LearningStyle       string                   `json:"learningStyle,omitempty" bson:"learningStyle,omitempty"` // e.g., visual, auditory, kinesthetic
	TopicPreferences    []string                 `json:"topicPreferences,omitempty" bson:"topicPreferences,omitempty"`
	SkillFocus          []string                 `json:"skillFocus,omitempty" bson:"skillFocus,omitempty"`
	SessionDuration     SessionDurationSettings  `json:"sessionDuration" bson:"sessionDuration"`
	SpacedRepetition    SpacedRepetitionSettings `json:"spacedRepetition" bson:"spacedRepetition"`
	Reminders           ReminderSettings         `json:"reminders" bson:"reminders"`
	Custom              map[string]interface{}   `json:"custom,omitempty" bson:"custom,omitempty"`
}

// SessionDurationSettings contains preferences for learning session lengths
type SessionDurationSettings struct {
	PreferredLength  int  `json:"preferredLength" bson:"preferredLength"` // Minutes
	EnableBreaks     bool `json:"enableBreaks" bson:"enableBreaks"`
	BreakFrequency   int  `json:"breakFrequency" bson:"breakFrequency"` // Minutes between breaks
	BreakDuration    int  `json:"breakDuration" bson:"breakDuration"`   // Minutes per break
	ExtendedSessions bool `json:"extendedSessions" bson:"extendedSessions"`
}

// SpacedRepetitionSettings contains spaced repetition algorithm preferences
type SpacedRepetitionSettings struct {
	Enabled           bool    `json:"enabled" bson:"enabled"`
	Algorithm         string  `json:"algorithm,omitempty" bson:"algorithm,omitempty"` // e.g., "sm2", "anki", "custom"
	BaseEaseFactor    float64 `json:"baseEaseFactor" bson:"baseEaseFactor"`           // Default: 2.5
	IntervalModifier  float64 `json:"intervalModifier" bson:"intervalModifier"`       // Default: 1.0
	MaximumInterval   int     `json:"maximumInterval" bson:"maximumInterval"`         // Days
	NewCardsPerDay    int     `json:"newCardsPerDay" bson:"newCardsPerDay"`
	ReviewCardsPerDay int     `json:"reviewCardsPerDay" bson:"reviewCardsPerDay"`
}

// ReminderSettings contains settings for learning reminders
type ReminderSettings struct {
	Enabled         bool     `json:"enabled" bson:"enabled"`
	Frequency       string   `json:"frequency" bson:"frequency"`           // daily, weekdays, custom
	Times           []string `json:"times" bson:"times"`                   // Format: "HH:MM" in 24h
	Days            []string `json:"days,omitempty" bson:"days,omitempty"` // If frequency is custom
	SmartReminders  bool     `json:"smartReminders" bson:"smartReminders"` // Adaptive reminders
	MissedGoalAlert bool     `json:"missedGoalAlert" bson:"missedGoalAlert"`
}
