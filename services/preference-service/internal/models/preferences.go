package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Preferences represents the complete set of user preferences
type Preferences struct {
	ID                       primitive.ObjectID       `json:"id,omitempty" bson:"_id,omitempty"`
	UserID                   string                   `json:"userId" bson:"userId"`
	UIPreferences            UIPreferences            `json:"uiPreferences" bson:"uiPreferences"`
	NotificationPreferences  NotificationPreferences  `json:"notificationPreferences" bson:"notificationPreferences"`
	LearningPreferences      LearningPreferences      `json:"learningPreferences" bson:"learningPreferences"`
	AccessibilityPreferences AccessibilityPreferences `json:"accessibilityPreferences" bson:"accessibilityPreferences"`
	SystemPreferences        SystemPreferences        `json:"systemPreferences" bson:"systemPreferences"`
	Metadata                 Metadata                 `json:"metadata" bson:"metadata"`
}

// Metadata contains system-generated preference information
type Metadata struct {
	CreatedAt    time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty" bson:"lastSyncedAt,omitempty"`
	Version      int        `json:"version" bson:"version"` // For optimistic concurrency control
}

// PreferenceKey represents a single preference key-value setting
type PreferenceKey struct {
	Key          string      `json:"key" bson:"key"`
	Value        interface{} `json:"value" bson:"value"`
	DefaultValue interface{} `json:"defaultValue,omitempty" bson:"defaultValue,omitempty"`
	Description  string      `json:"description,omitempty" bson:"description,omitempty"`
	Type         string      `json:"type,omitempty" bson:"type,omitempty"` // string, number, boolean, etc.
}

// CustomPreferences allows for storing arbitrary preferences
type CustomPreferences struct {
	Items map[string]interface{} `json:"items" bson:"items"`
}
