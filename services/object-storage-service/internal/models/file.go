package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// File represents a file stored in the system
type File struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	OwnerID        string             `bson:"ownerId" json:"ownerId"`                                   // User ID of the owner
	Name           string             `bson:"name" json:"name"`                                         // Original filename
	Description    string             `bson:"description" json:"description"`                           // File description
	Size           int64              `bson:"size" json:"size"`                                         // Size in bytes
	ContentType    string             `bson:"contentType" json:"contentType"`                           // MIME type
	StoragePath    string             `bson:"storagePath" json:"storagePath"`                           // Path in MinIO
	BucketName     string             `bson:"bucketName" json:"bucketName"`                             // MinIO bucket name
	IsPublic       bool               `bson:"isPublic" json:"isPublic"`                                 // Public access flag
	Checksum       string             `bson:"checksum" json:"checksum"`                                 // MD5 checksum
	VersionCount   int                `bson:"versionCount" json:"versionCount"`                         // Number of versions
	CurrentVersion string             `bson:"currentVersion" json:"currentVersion"`                     // Current version ID
	FolderPath     string             `bson:"folderPath" json:"folderPath"`                             // Virtual folder path
	Tags           []string           `bson:"tags,omitempty" json:"tags,omitempty"`                     // Searchable tags
	Metadata       map[string]string  `bson:"metadata,omitempty" json:"metadata,omitempty"`             // Custom metadata
	Permissions    []Permission       `bson:"permissions,omitempty" json:"permissions,omitempty"`       // Access permissions
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`                               // Creation timestamp
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`                               // Last update timestamp
	LastAccessedAt *time.Time         `bson:"lastAccessedAt,omitempty" json:"lastAccessedAt,omitempty"` // Last access time
}

// FileVersion represents a version of a file
type FileVersion struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	FileID        primitive.ObjectID `bson:"fileId" json:"fileId"`               // Reference to the parent file
	VersionNumber int                `bson:"versionNumber" json:"versionNumber"` // Version number
	Size          int64              `bson:"size" json:"size"`                   // Size in bytes
	StoragePath   string             `bson:"storagePath" json:"storagePath"`     // Path in MinIO
	Checksum      string             `bson:"checksum" json:"checksum"`           // MD5 checksum
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`         // Creation timestamp
	CreatedBy     string             `bson:"createdBy" json:"createdBy"`         // User ID who created this version
}
