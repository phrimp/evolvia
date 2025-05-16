package events

import (
	"encoding/json"
	utils "proto-gen/utils"
	"time"
)

type EventType string

const (
	// UserRegister is triggered when a new user is registered
	UserRegister EventType = "user.registered"
	// ProfileUpdated is triggered when a user profile is updated
	ProfileUpdated EventType = "profile.updated"
)

type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Version   string    `json:"version"`
}

type UserRegisterEvent struct {
	BaseEvent
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	ProfileData map[string]string `json:"profile_data"`
}

func NewUserRegisterEvent(userID, username, email string, profileData map[string]string) *UserRegisterEvent {
	return &UserRegisterEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      UserRegister,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID:      userID,
		Username:    username,
		Email:       email,
		ProfileData: profileData,
	}
}

func (e *UserRegisterEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

type ProfileUpdatedEvent struct {
	BaseEvent
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

func NewProfileUpdatedEvent(userID, username string) *ProfileUpdatedEvent {
	return &ProfileUpdatedEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      ProfileUpdated,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID:   userID,
		Username: username,
	}
}

// ToJSON serializes the event to JSON
func (e *ProfileUpdatedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// generateEventID generates a unique ID for an event
func generateEventID() string {
	return time.Now().Format("20060102150405") + "-" + utils.GenerateRandomStringWithLength(6)
}
