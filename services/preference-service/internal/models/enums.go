package models

type PreferenceType string

const (
	PreferenceTypeUI            PreferenceType = "ui"
	PreferenceTypeNotification  PreferenceType = "notification"
	PreferenceTypeLearning      PreferenceType = "learning"
	PreferenceTypeAccessibility PreferenceType = "accessibility"
	PreferenceTypeSystem        PreferenceType = "system"
)

type ThemeMode string

const (
	ThemeModeLight  ThemeMode = "light"
	ThemeModeDark   ThemeMode = "dark"
	ThemeModeSystem ThemeMode = "system" // Follows system settings
)

// NotificationFrequency represents how often notifications are received
type NotificationFrequency string

// NotificationFrequency enum values
const (
	NotificationFrequencyRealtime NotificationFrequency = "realtime"
	NotificationFrequencyHourly   NotificationFrequency = "hourly"
	NotificationFrequencyDaily    NotificationFrequency = "daily"
	NotificationFrequencyWeekly   NotificationFrequency = "weekly"
	NotificationFrequencyNever    NotificationFrequency = "never"
)

// LearningPace represents preferred pace for learning content
type LearningPace string

// LearningPace enum values
const (
	LearningPaceSlow     LearningPace = "slow"
	LearningPaceModerate LearningPace = "moderate"
	LearningPaceFast     LearningPace = "fast"
	LearningPaceCustom   LearningPace = "custom" // Customized pace
)

// DifficultyLevel represents preferred content difficulty
type DifficultyLevel string

// DifficultyLevel enum values
const (
	DifficultyLevelBeginner     DifficultyLevel = "beginner"
	DifficultyLevelIntermediate DifficultyLevel = "intermediate"
	DifficultyLevelAdvanced     DifficultyLevel = "advanced"
	DifficultyLevelExpert       DifficultyLevel = "expert"
)

// ContentFormat represents preferred content format
type ContentFormat string

// ContentFormat enum values
const (
	ContentFormatText        ContentFormat = "text"
	ContentFormatAudio       ContentFormat = "audio"
	ContentFormatVideo       ContentFormat = "video"
	ContentFormatInteractive ContentFormat = "interactive"
)
