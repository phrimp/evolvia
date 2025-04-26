package repository

import (
	"auth_service/internal/models"
	"context"
	"time"
)

type SessionRepository struct{}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{}
}

func (r *SessionRepository) Create(ctx context.Context, session *models.Session) (*models.Session, error) {
	currentTime := int(time.Now().Unix())

	if session.CreatedAt == 0 {
		session.CreatedAt = currentTime
	}

	if session.LastActivityAt == 0 {
		session.LastActivityAt = currentTime
	}

	return session, nil
}
