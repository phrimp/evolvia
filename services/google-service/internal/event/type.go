package event

import "time"

type EventType string

const (
	EventTypeGoogleLogin              EventType = "google.login"
	EventTypeGoogleLoginRequest       EventType = "google.login.request"
	EventTypeGoogleLoginResponse      EventType = "google.login.response"
	EventTypeEmailVerificationSuccess EventType = "email.verification.success"
)

type BaseEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Version   string    `json:"version"`
}

type GoogleLoginEvent struct {
	BaseEvent
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Locale string `json:"locale"`
}

type EmailVerificationSuccessEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	Email  string `json:"email"`
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

func NewGoogleLoginEvent(email, name, avatar, locale string) *GoogleLoginEvent {
	return &GoogleLoginEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeGoogleLogin,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		Email:  email,
		Name:   name,
		Avatar: avatar,
		Locale: locale,
	}
}

func NewEmailVerificationSuccessEvent(userID, email string) *EmailVerificationSuccessEvent {
	return &EmailVerificationSuccessEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeEmailVerificationSuccess,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID: userID,
		Email:  email,
	}
}

func NewGoogleLoginRequestEvent(email, name, picture, googleID, locale string, profile map[string]string) *GoogleLoginRequestEvent {
	return &GoogleLoginRequestEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeGoogleLoginRequest,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		RequestID: generateEventID(), // Unique request ID for correlation
		Email:     email,
		Name:      name,
		Picture:   picture,
		GoogleID:  googleID,
		Locale:    locale,
		Profile:   profile,
	}
}

func NewGoogleLoginResponseEvent(requestID string, success bool, sessionToken, errorMsg, userID string) *GoogleLoginResponseEvent {
	return &GoogleLoginResponseEvent{
		BaseEvent: BaseEvent{
			ID:        generateEventID(),
			Type:      EventTypeGoogleLoginResponse,
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

func generateEventID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
