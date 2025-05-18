package models

import "time"

type AccessLevel string

const (
	AccessLevelRead  AccessLevel = "read"
	AccessLevelWrite AccessLevel = "write"
	AccessLevelAdmin AccessLevel = "admin"
	AccessLevelNone  AccessLevel = "none"
)

type Permission struct {
	EntityID    string      `bson:"entityId" json:"entityId"`                       // User or group ID
	EntityType  string      `bson:"entityType" json:"entityType"`                   // "user" or "group"
	AccessLevel AccessLevel `bson:"accessLevel" json:"accessLevel"`                 // Level of access
	GrantedBy   string      `bson:"grantedBy" json:"grantedBy"`                     // Who granted this permission
	GrantedAt   time.Time   `bson:"grantedAt" json:"grantedAt"`                     // When the permission was granted
	ExpiresAt   *time.Time  `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"` // Expiration time (optional)
}
