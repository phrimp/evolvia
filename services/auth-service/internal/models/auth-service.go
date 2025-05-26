package models

import (
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserAuth struct {
	ID                  bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email               string        `bson:"email" json:"email" validate:"required,email"`
	Username            string        `bson:"username,omitempty" json:"username"`
	PasswordHash        string        `bson:"passwordHash" json:"passwordHash"`
	IsActive            bool          `bson:"isActive" json:"isActive"`
	IsEmailVerified     bool          `bson:"isEmailVerified" json:"isEmailVerified"`
	FailedLoginAttempts int           `bson:"failedLoginAttempts" json:"failedLoginAttempts"`
	LastLoginAttempt    int           `bson:"lastLoginAttempt,omitempty" json:"lastLoginAttempt"`
	CreatedAt           int           `bson:"createdAt" json:"createdAt"`
	UpdatedAt           int           `bson:"updatedAt" json:"updatedAt"`
	BasicProfile        UserProfile   `bson:"basicProfile" json:"basicProfile"`
	LastLoginAt         int           `bson:"lastLoginAt,omitempty" json:"lastLoginAt"`
}

type Session struct {
	Token          string   `bson:"token" json:"token"`
	IPAddress      string   `bson:"ipAddress" json:"ipAddress"`
	IsValid        bool     `bson:"isValid" json:"isValid"`
	CreatedAt      int      `bson:"createdAt" json:"createdAt"`
	LastActivityAt int      `bson:"lastActivityAt" json:"lastActivityAt"`
	Device         Device   `bson:"device,omitempty" json:"device"`
	Location       Location `bson:"location,omitempty" json:"location"`
}

type Device struct {
	Type    string `bson:"type,omitempty" json:"type"`
	OS      string `bson:"os,omitempty" json:"os"`
	Browser string `bson:"browser,omitempty" json:"browser"`
}

type Location struct {
	Country string `bson:"country,omitempty" json:"country"`
	Region  string `bson:"region,omitempty" json:"region"`
	City    string `bson:"city,omitempty" json:"city"`
}

type Role struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name" validate:"required"`
	Description string        `bson:"description,omitempty" json:"description"`
	Permissions []string      `bson:"permissions" json:"permissions"`
	IsSystem    bool          `bson:"isSystem" json:"isSystem"`
	CreatedAt   int           `bson:"createdAt" json:"createdAt"`
	UpdatedAt   int           `bson:"updatedAt" json:"updatedAt"`
}

type UserRole struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     bson.ObjectID `bson:"userId" json:"userId"`
	RoleID     bson.ObjectID `bson:"roleId" json:"roleId"`
	ScopeType  string        `bson:"scopeType,omitempty" json:"scopeType"`
	ScopeID    bson.ObjectID `bson:"scopeId,omitempty" json:"scopeId"`
	AssignedBy bson.ObjectID `bson:"assignedBy" json:"assignedBy"`
	AssignedAt int           `bson:"assignedAt" json:"assignedAt"`
	ExpiresAt  int           `bson:"expiresAt,omitempty" json:"expiresAt"`
	IsActive   bool          `bson:"isActive" json:"isActive"`
}

type Permission struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name" validate:"required"`
	Description string        `bson:"description,omitempty" json:"description"`
	Category    string        `bson:"category,omitempty" json:"category"`
	IsSystem    bool          `bson:"isSystem" json:"isSystem"`
	CreatedAt   int           `bson:"createdAt" json:"createdAt"`
	UpdatedAt   int           `bson:"updatedAt" json:"updatedAt"`
}

type AuditLog struct {
	ID        bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID    bson.ObjectID  `bson:"userId" json:"userId"`
	Event     string         `bson:"event" json:"event"`
	IPAddress string         `bson:"ipAddress" json:"ipAddress"`
	UserAgent string         `bson:"userAgent" json:"userAgent"`
	Details   map[string]any `bson:"details,omitempty" json:"details"`
	Timestamp int            `bson:"timestamp" json:"timestamp"`
}

type Claims struct {
	jwt.RegisteredClaims
	Id          string
	Username    string
	Email       string
	Permissions []string
}
