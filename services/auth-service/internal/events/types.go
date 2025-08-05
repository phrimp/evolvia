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
	UserLogin    EventType = "user.login"
	// ProfileUpdated is triggered when a user profile is updated
	ProfileUpdated           EventType = "profile.updated"
	GoogleLoginRequest       EventType = "google.login.request"
	GoogleLoginResponse      EventType = "google.login.response"
	EmailVerificationSuccess EventType = "email.verification.success"
)

type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Version   string    `json:"version"`
}
type EmailVerificationSuccessEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type GoogleLoginEvent struct {
	BaseEvent
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Locale string `json:"locale"`
}

type GoogleLoginRequestEvent struct {
	BaseEvent
	RequestID string            `json:"request_id"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	Picture   string            `json:"picture"`
	GoogleID  string            `json:"google_id"`
	Locale    string            `json:"locale"`
	Profile   map[string]string `json:"profile"`
}

type GoogleLoginResponseEvent struct {
	BaseEvent
	RequestID    string `json:"request_id"`
	Success      bool   `json:"success"`
	SessionToken string `json:"session_token,omitempty"`
	Error        string `json:"error,omitempty"`
	UserID       string `json:"user_id,omitempty"`
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

type UserLoginEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
}

func NewUserLoginEvent(userID string) *UserLoginEvent {
	return &UserLoginEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      UserLogin,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID: userID,
	}
}

func (e *UserLoginEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
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

func NewGoogleLoginResponseEvent(requestID string, success bool, sessionToken, errorMsg, userID string) *GoogleLoginResponseEvent {
	return &GoogleLoginResponseEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      GoogleLoginResponse,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		RequestID:    requestID,
		Success:      success,
		SessionToken: sessionToken,
		Error:        errorMsg,
		UserID:       userID,
	}
}

func (e *GoogleLoginResponseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// generateEventID generates a unique ID for an event
func generateEventID() string {
	return time.Now().Format("20060102150405") + "-" + utils.GenerateRandomStringWithLength(6)
}

type PlanCreatedEvent struct {
	EventType    string                `json:"eventType"`
	PlanID       string                `json:"planId"`
	PlanName     string                `json:"planName"`
	PlanType     string                `json:"planType"`
	Price        float64               `json:"price"`
	Currency     string                `json:"currency"`
	BillingCycle string                `json:"billingCycle"`
	Features     []FeatureDetail       `json:"features"`
	IsActive     bool                  `json:"isActive"`
	TrialDays    int                   `json:"trialDays"`
	Timestamp    int64                 `json:"timestamp"`
	RoleMetadata *RoleCreationMetadata `json:"roleMetadata,omitempty"`
}

type FeatureDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Permissions []string `json:"permissions"`
}

type RoleCreationMetadata struct {
	SuggestedRoleName    string              `json:"suggestedRoleName"`
	AllPermissions       []string            `json:"allPermissions"`
	FeaturePermissionMap map[string][]string `json:"featurePermissionMap"`
	RoleDescription      string              `json:"roleDescription"`
}

type PlanUpdatedEvent struct {
	EventType     string                `json:"eventType"`
	PlanID        string                `json:"planId"`
	PlanName      string                `json:"planName"`
	PlanType      string                `json:"planType"`
	Price         float64               `json:"price"`
	Currency      string                `json:"currency"`
	BillingCycle  string                `json:"billingCycle"`
	Features      []FeatureDetail       `json:"features"`
	IsActive      bool                  `json:"isActive"`
	TrialDays     int                   `json:"trialDays"`
	Timestamp     int64                 `json:"timestamp"`
	ChangedFields []string              `json:"changedFields,omitempty"`
	OldValues     map[string]any        `json:"oldValues,omitempty"`
	NewValues     map[string]any        `json:"newValues,omitempty"`
	RoleMetadata  *RoleCreationMetadata `json:"roleMetadata,omitempty"`
}

type PlanDeletedEvent struct {
	EventType    string                `json:"eventType"`
	PlanID       string                `json:"planId"`
	PlanName     string                `json:"planName"`
	PlanType     string                `json:"planType"`
	Timestamp    int64                 `json:"timestamp"`
	RoleMetadata *RoleCreationMetadata `json:"roleMetadata,omitempty"`
}

type SubscriptionEvent struct {
	EventType      string         `json:"eventType"`
	SubscriptionID string         `json:"subscriptionId"`
	UserID         string         `json:"userId"`
	PlanID         string         `json:"planId"`
	PlanName       string         `json:"planName,omitempty"`
	PlanType       string         `json:"planType,omitempty"`
	Status         string         `json:"status"`
	Timestamp      int64          `json:"timestamp"`
	ChangedFields  []string       `json:"changedFields,omitempty"`
	OldValues      map[string]any `json:"oldValues,omitempty"`
	NewValues      map[string]any `json:"newValues,omitempty"`

	// User role assignment metadata
	UserRoleMetadata *UserRoleMetadata `json:"userRoleMetadata,omitempty"`
}

type UserRoleMetadata struct {
	ShouldAssignRole bool     `json:"shouldAssignRole"`
	RoleName         string   `json:"roleName"`
	Permissions      []string `json:"permissions"`
	PreviousRoles    []string `json:"previousRoles,omitempty"`
}
