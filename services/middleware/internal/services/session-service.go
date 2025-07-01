package services

import (
	"context"
	"log"
	"middleware/internal/models"
	"middleware/internal/repository"
)

type SessionService struct {
	jwtService *JWTService
}

func NewSessionService(jwtService *JWTService) *SessionService {
	return &SessionService{
		jwtService: jwtService,
	}
}

func (s *SessionService) GetSession(ctx context.Context, token string) (*models.Session, error) {
	session := &models.Session{}
	err := repository.Redis_repo.GetStructCached(ctx, token, session)
	if err != nil {
		log.Printf("Error retrieving session: %s", err)
		return nil, err
	}
	return session, nil
}

func (s *SessionService) CheckSystemStatus(ctx context.Context) (bool, error) {
	var status bool
	err := repository.Redis_repo.GetStructCached(ctx, "maintenance", &status)
	if err != nil {
		log.Printf("System Maintenance Status not found or not activated: %s", err)
		return false, err
	}
	return status, nil
}

func (s *SessionService) ValidateToken(token string) (*models.Claims, error) {
	return s.jwtService.VerifyToken(token)
}
