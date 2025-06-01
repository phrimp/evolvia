package event

import "time"

type EventType string

const (
	EventTypeGoogleLogin EventType = "google.login"
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
