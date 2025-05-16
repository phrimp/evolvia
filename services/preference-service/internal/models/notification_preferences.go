package models

type NotificationPreferences struct {
	EnableNotifications bool                        `json:"enableNotifications" bson:"enableNotifications"`
	DefaultFrequency    NotificationFrequency       `json:"defaultFrequency" bson:"defaultFrequency"`
	Channels            NotificationChannels        `json:"channels" bson:"channels"`
	Categories          map[string]CategorySettings `json:"categories" bson:"categories"` // Category-specific settings
	QuietHours          QuietHoursSettings          `json:"quietHours,omitempty" bson:"quietHours,omitempty"`
	Custom              map[string]interface{}      `json:"custom,omitempty" bson:"custom,omitempty"`
}

// NotificationChannels represents which notification channels are enabled
type NotificationChannels struct {
	InApp       bool `json:"inApp" bson:"inApp"`
	Email       bool `json:"email" bson:"email"`
	PushMobile  bool `json:"pushMobile" bson:"pushMobile"`
	PushDesktop bool `json:"pushDesktop" bson:"pushDesktop"`
	SMS         bool `json:"sms" bson:"sms"`
}

// CategorySettings contains notification settings for a specific category
type CategorySettings struct {
	Enabled     bool                  `json:"enabled" bson:"enabled"`
	Frequency   NotificationFrequency `json:"frequency" bson:"frequency"`
	Importance  int                   `json:"importance" bson:"importance"` // 1-5 scale
	Channels    NotificationChannels  `json:"channels,omitempty" bson:"channels,omitempty"`
	CustomRules map[string]bool       `json:"customRules,omitempty" bson:"customRules,omitempty"`
}

// QuietHoursSettings defines when notifications should be suppressed
type QuietHoursSettings struct {
	Enabled     bool     `json:"enabled" bson:"enabled"`
	StartTime   string   `json:"startTime" bson:"startTime"`   // Format: "HH:MM" in 24h
	EndTime     string   `json:"endTime" bson:"endTime"`       // Format: "HH:MM" in 24h
	ActiveDays  []string `json:"activeDays" bson:"activeDays"` // e.g., ["monday", "tuesday"]
	AllowUrgent bool     `json:"allowUrgent" bson:"allowUrgent"`
}
