package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Avatar struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID      string             `bson:"userId" json:"userId"`           // User ID of the owner
	FileName    string             `bson:"fileName" json:"fileName"`       // Original filename
	Size        int64              `bson:"size" json:"size"`               // Size in bytes
	ContentType string             `bson:"contentType" json:"contentType"` // MIME type
	StoragePath string             `bson:"storagePath" json:"storagePath"` // Path in MinIO
	BucketName  string             `bson:"bucketName" json:"bucketName"`   // MinIO bucket name
	IsDefault   bool               `bson:"isDefault" json:"isDefault"`     // Whether this is the default avatar
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`     // Creation timestamp
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`     // Last update timestamp
}
