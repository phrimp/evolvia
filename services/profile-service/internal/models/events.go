package models

type EventType string

const (
	EventTypeProfileCreated      EventType = "profile.created"
	EventTypeProfileUpdated      EventType = "profile.updated"
	EventTypeProfileDeleted      EventType = "profile.deleted"
	EventTypeCompletenessChanged EventType = "profile.completeness.changed"
	EventTypeUserRegistered      EventType = "user.registered"
)

type ProfileEvent struct {
	EventType     EventType      `json:"eventType"`
	ProfileID     string         `json:"profileId"`
	UserID        string         `json:"userId"`
	Timestamp     int            `json:"timestamp"`
	ChangedFields []string       `json:"changedFields,omitempty"`
	OldValues     map[string]any `json:"oldValues,omitempty"`
	NewValues     map[string]any `json:"newValues,omitempty"`
}

type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Version   string    `json:"version"`
}

type UserRegisterEvent struct {
	BaseEvent
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	ProfileData map[string]string `json:"profile_data"`
}
