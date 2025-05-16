package models

import (
	"time"
)

type EventType string

const (
	EventTypeProfileCreated      EventType = "profile.created"
	EventTypeProfileUpdated      EventType = "profile.updated"
	EventTypeProfileDeleted      EventType = "profile.deleted"
	EventTypeCompletenessChanged EventType = "profile.completeness.changed"
)

type ProfileEvent struct {
	EventType     EventType      `json:"eventType"`
	ProfileID     string         `json:"profileId"`
	UserID        string         `json:"userId"`
	Timestamp     time.Time      `json:"timestamp"`
	ChangedFields []string       `json:"changedFields,omitempty"`
	OldValues     map[string]any `json:"oldValues,omitempty"`
	NewValues     map[string]any `json:"newValues,omitempty"`
}
