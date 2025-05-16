package models

// PreferenceDTO is a Data Transfer Object for preference updates
type PreferenceDTO struct {
	UIPreferences            *UIPreferences            `json:"uiPreferences,omitempty"`
	NotificationPreferences  *NotificationPreferences  `json:"notificationPreferences,omitempty"`
	LearningPreferences      *LearningPreferences      `json:"learningPreferences,omitempty"`
	AccessibilityPreferences *AccessibilityPreferences `json:"accessibilityPreferences,omitempty"`
	SystemPreferences        *SystemPreferences        `json:"systemPreferences,omitempty"`
}

// CreatePreferencesRequest represents the request to create new preferences
type CreatePreferencesRequest struct {
	UserID                   string                   `json:"userId" binding:"required"`
	UIPreferences            UIPreferences            `json:"uiPreferences"`
	NotificationPreferences  NotificationPreferences  `json:"notificationPreferences"`
	LearningPreferences      LearningPreferences      `json:"learningPreferences"`
	AccessibilityPreferences AccessibilityPreferences `json:"accessibilityPreferences"`
	SystemPreferences        SystemPreferences        `json:"systemPreferences"`
}

// UpdatePreferencesRequest represents the request to update existing preferences
type UpdatePreferencesRequest struct {
	PreferenceDTO PreferenceDTO `json:"preferences" binding:"required"`
}

// UpdateCategoryRequest represents a request to update a specific preference category
type UpdateCategoryRequest struct {
	Category PreferenceType `json:"category" binding:"required"`
	Data     interface{}    `json:"data" binding:"required"`
}

// SinglePreferenceRequest represents a request to update a single preference
type SinglePreferenceRequest struct {
	Path  string      `json:"path" binding:"required"` // Dot-notation path to preference
	Value interface{} `json:"value" binding:"required"`
}

// BulkPreferencesRequest represents multiple preference updates in one request
type BulkPreferencesRequest struct {
	Updates []SinglePreferenceRequest `json:"updates" binding:"required,min=1"`
}

// PreferenceSearchQuery represents search parameters for preferences
type PreferenceSearchQuery struct {
	UserIDs  []string `form:"userIds"`
	Keys     []string `form:"keys"`
	Page     int      `form:"page,default=1"`
	PageSize int      `form:"pageSize,default=20"`
}

// PreferenceSearchResult represents a paginated search result
type PreferenceSearchResult struct {
	Preferences []Preferences `json:"preferences"`
	TotalCount  int64         `json:"totalCount"`
	PageCount   int           `json:"pageCount"`
	CurrentPage int           `json:"currentPage"`
}
