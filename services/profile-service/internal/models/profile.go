package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PersonalInfo struct {
	FirstName   string     `json:"firstName" bson:"firstName"`
	LastName    string     `json:"lastName" bson:"lastName"`
	DisplayName string     `json:"displayName,omitempty" bson:"displayName,omitempty"`
	DateOfBirth *time.Time `json:"dateOfBirth,omitempty" bson:"dateOfBirth,omitempty"`
	Gender      Gender     `json:"gender,omitempty" bson:"gender,omitempty"`
	Biography   string     `json:"biography,omitempty" bson:"biography,omitempty"`
	Interests   []string   `json:"interests,omitempty" bson:"interests,omitempty"`
}

type Metadata struct {
	CreatedAt time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updatedAt"`
	LastLogin *time.Time `json:"lastLogin,omitempty" bson:"lastLogin,omitempty"`
}

type Profile struct {
	ID                    primitive.ObjectID      `json:"id,omitempty" bson:"_id,omitempty"`
	UserID                string                  `json:"userId" bson:"userId"`
	PersonalInfo          PersonalInfo            `json:"personalInfo" bson:"personalInfo"`
	ContactInfo           ContactInfo             `json:"contactInfo" bson:"contactInfo"`
	EducationalBackground []EducationalBackground `json:"educationalBackground,omitempty" bson:"educationalBackground,omitempty"`
	AvatarURL             string                  `json:"avatarUrl,omitempty" bson:"avatarUrl,omitempty"`
	PrivacySettings       PrivacySettings         `json:"privacySettings" bson:"privacySettings"`
	ProfileCompleteness   float64                 `json:"profileCompleteness" bson:"profileCompleteness"`
	Metadata              Metadata                `json:"metadata" bson:"metadata"`
}
