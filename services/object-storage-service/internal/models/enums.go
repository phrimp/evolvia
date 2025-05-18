package models

type FileStatus string

const (
	FileStatusActive      FileStatus = "active"
	FileStatusDeleted     FileStatus = "deleted"
	FileStatusArchived    FileStatus = "archived"
	FileStatusQuarantined FileStatus = "quarantined" // For files that failed virus scan
)

type EntityType string

const (
	EntityTypeUser   EntityType = "user"
	EntityTypeGroup  EntityType = "group"
	EntityTypeRole   EntityType = "role"
	EntityTypePublic EntityType = "public"
)

type EventType string

const (
	EventTypeFileUploaded   EventType = "file.uploaded"
	EventTypeFileUpdated    EventType = "file.updated"
	EventTypeFileDeleted    EventType = "file.deleted"
	EventTypeFileAccessed   EventType = "file.accessed"
	EventTypeAvatarUploaded EventType = "avatar.uploaded"
	EventTypeAvatarUpdated  EventType = "avatar.updated"
	EventTypeAvatarDeleted  EventType = "avatar.deleted"
)
