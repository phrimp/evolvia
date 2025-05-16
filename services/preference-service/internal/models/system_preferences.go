package models

type SystemPreferences struct {
	Language             string                 `json:"language" bson:"language"`                   // ISO language code
	TimeZone             string                 `json:"timeZone" bson:"timeZone"`                   // IANA time zone
	DateFormat           string                 `json:"dateFormat" bson:"dateFormat"`               // e.g., "YYYY-MM-DD"
	TimeFormat           string                 `json:"timeFormat" bson:"timeFormat"`               // 12h or 24h
	StartDayOfWeek       int                    `json:"startDayOfWeek" bson:"startDayOfWeek"`       // 0 = Sunday, 1 = Monday
	MeasurementSystem    string                 `json:"measurementSystem" bson:"measurementSystem"` // metric or imperial
	EnableDataCollection bool                   `json:"enableDataCollection" bson:"enableDataCollection"`
	EnableAnalytics      bool                   `json:"enableAnalytics" bson:"enableAnalytics"`
	DefaultPrivacy       string                 `json:"defaultPrivacy" bson:"defaultPrivacy"` // public, private, connections
	SyncSettings         SyncSettings           `json:"syncSettings" bson:"syncSettings"`
	Security             SecuritySettings       `json:"security" bson:"security"`
	Custom               map[string]interface{} `json:"custom,omitempty" bson:"custom,omitempty"`
}

// SyncSettings contains preferences for data synchronization
type SyncSettings struct {
	EnableAutoSync       bool `json:"enableAutoSync" bson:"enableAutoSync"`
	SyncOnWifiOnly       bool `json:"syncOnWifiOnly" bson:"syncOnWifiOnly"`
	SyncFrequencyMinutes int  `json:"syncFrequencyMinutes" bson:"syncFrequencyMinutes"`
	BackgroundSync       bool `json:"backgroundSync" bson:"backgroundSync"`
	SyncMedia            bool `json:"syncMedia" bson:"syncMedia"`
}

// SecuritySettings contains security-related preferences
type SecuritySettings struct {
	EnableTwoFactor       bool `json:"enableTwoFactor" bson:"enableTwoFactor"`
	SessionTimeoutMinutes int  `json:"sessionTimeoutMinutes" bson:"sessionTimeoutMinutes"`
	RememberDevices       bool `json:"rememberDevices" bson:"rememberDevices"`
	RequirePasswordReset  int  `json:"requirePasswordReset" bson:"requirePasswordReset"` // Days, 0 = never
	NotifyOnNewLogin      bool `json:"notifyOnNewLogin" bson:"notifyOnNewLogin"`
	DeviceVerification    bool `json:"deviceVerification" bson:"deviceVerification"`
}
