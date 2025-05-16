package models

import (
	"time"
)

// PreferenceEventType represents the type of preference event
type PreferenceEventType string

// PreferenceEventType enum values
const (
	PreferenceEventTypeCreated         PreferenceEventType = "preferences.created"
	PreferenceEventTypeUpdated         PreferenceEventType = "preferences.updated"
	PreferenceEventTypeDeleted         PreferenceEventType = "preferences.deleted"
	PreferenceEventTypeCategoryUpdated PreferenceEventType = "preferences.category.updated"
	PreferenceEventTypeThemeChanged    PreferenceEventType = "preferences.theme.changed"
	PreferenceEventTypeSynced          PreferenceEventType = "preferences.synced"
)

// PreferenceEvent represents an event related to preference changes
type PreferenceEvent struct {
	EventType    PreferenceEventType    `json:"eventType"`
	UserID       string                 `json:"userId"`
	Timestamp    time.Time              `json:"timestamp"`
	Category     PreferenceType         `json:"category,omitempty"`
	ChangedPaths []string               `json:"changedPaths,omitempty"`
	OldValues    map[string]interface{} `json:"oldValues,omitempty"`
	NewValues    map[string]interface{} `json:"newValues,omitempty"`
	Source       string                 `json:"source,omitempty"`   // "api", "sync", "default", etc.
	ClientID     string                 `json:"clientId,omitempty"` // Device/client that made the change
	Version      int                    `json:"version"`
}

// ThemeChangeEvent represents a specific event for theme changes
type ThemeChangeEvent struct {
	UserID      string    `json:"userId"`
	OldTheme    ThemeMode `json:"oldTheme"`
	NewTheme    ThemeMode `json:"newTheme"`
	Timestamp   time.Time `json:"timestamp"`
	AutoChanged bool      `json:"autoChanged,omitempty"` // Changed automatically (e.g., system dark mode)
	ClientID    string    `json:"clientId,omitempty"`
}

// PreferenceSyncEvent represents synchronization of preferences
type PreferenceSyncEvent struct {
	UserID           string           `json:"userId"`
	Timestamp        time.Time        `json:"timestamp"`
	DeviceID         string           `json:"deviceId"`
	SyncedCategories []PreferenceType `json:"syncedCategories"`
	ConflictResolved bool             `json:"conflictResolved,omitempty"`
	Version          int              `json:"version"`
}
