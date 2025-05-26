package event

import (
	"profile-service/internal/models"
	"time"
)

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

func NewUserRegisteredEvent(userID, username, email string, profileData map[string]string) *models.UserRegisterEvent {
	return &models.UserRegisterEvent{
		BaseEvent: models.BaseEvent{
			ID:        generateEventID(),
			Type:      models.EventTypeUserRegistered,
			Timestamp: time.Now().Unix(),
			Version:   "1.0",
		},
		UserID:      userID,
		Username:    username,
		Email:       email,
		ProfileData: profileData,
	}
}
