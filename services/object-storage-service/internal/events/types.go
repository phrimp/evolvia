package events

import (
	"time"
)

type EventType string

const (
	// File events
	EventTypeFileUploaded EventType = "file.uploaded"
	EventTypeFileUpdated  EventType = "file.updated"
	EventTypeFileDeleted  EventType = "file.deleted"
	EventTypeFileAccessed EventType = "file.accessed"

	// Avatar events
	EventTypeAvatarUploaded EventType = "avatar.uploaded"
	EventTypeAvatarUpdated  EventType = "avatar.updated"
	EventTypeAvatarDeleted  EventType = "avatar.deleted"

	EventTypeUserRegistered EventType = "user.registered"
)

// BaseEvent represents the common fields for all events
type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Version   string    `json:"version"`
}

// FileEvent represents an event related to a file operation
type FileEvent struct {
	BaseEvent
	FileID       string `json:"fileId"`
	OwnerID      string `json:"ownerId"`
	FileName     string `json:"fileName,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	FolderPath   string `json:"folderPath,omitempty"`
	IsPublic     bool   `json:"isPublic,omitempty"`
	VersionCount int    `json:"versionCount,omitempty"`
}

// AvatarEvent represents an event related to an avatar operation
type AvatarEvent struct {
	BaseEvent
	AvatarID    string `json:"avatarId"`
	UserID      string `json:"userId"`
	IsDefault   bool   `json:"isDefault,omitempty"`
	Size        int64  `json:"size,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// NewFileUploadedEvent creates a new file uploaded event
func NewFileUploadedEvent(fileID, ownerID, fileName string) *FileEvent {
	return &FileEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFileUploaded,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		FileID:   fileID,
		OwnerID:  ownerID,
		FileName: fileName,
	}
}

// NewFileUpdatedEvent creates a new file updated event
func NewFileUpdatedEvent(fileID, ownerID string) *FileEvent {
	return &FileEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFileUpdated,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		FileID:  fileID,
		OwnerID: ownerID,
	}
}

// NewFileDeletedEvent creates a new file deleted event
func NewFileDeletedEvent(fileID, ownerID string) *FileEvent {
	return &FileEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFileDeleted,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		FileID:  fileID,
		OwnerID: ownerID,
	}
}

// NewFileAccessedEvent creates a new file accessed event
func NewFileAccessedEvent(fileID, ownerID string) *FileEvent {
	return &FileEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeFileAccessed,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		FileID:  fileID,
		OwnerID: ownerID,
	}
}

// NewAvatarUploadedEvent creates a new avatar uploaded event
func NewAvatarUploadedEvent(avatarID, userID string) *AvatarEvent {
	return &AvatarEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeAvatarUploaded,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		AvatarID: avatarID,
		UserID:   userID,
	}
}

// NewAvatarUpdatedEvent creates a new avatar updated event
func NewAvatarUpdatedEvent(avatarID, userID string) *AvatarEvent {
	return &AvatarEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeAvatarUpdated,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		AvatarID: avatarID,
		UserID:   userID,
	}
}

// NewAvatarDeletedEvent creates a new avatar deleted event
func NewAvatarDeletedEvent(avatarID, userID string) *AvatarEvent {
	return &AvatarEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeAvatarDeleted,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		AvatarID: avatarID,
		UserID:   userID,
	}
}

// Helper function to generate a unique event ID
func generateEventID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + randomString(8)
}

// Helper function to generate a random string of a given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

type UserRegisterEvent struct {
	BaseEvent
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	ProfileData map[string]string `json:"profile_data"`
}

// Add constructor function
func NewUserRegisteredEvent(userID, username, email string, profileData map[string]string) *UserRegisterEvent {
	return &UserRegisterEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeUserRegistered,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID:      userID,
		Username:    username,
		Email:       email,
		ProfileData: profileData,
	}
}
