package service

import (
	"auth_service/internal/models"
	"auth_service/internal/repository"
	"context"
	"fmt"
	"log"
	"time"
)

type SessionService struct {
	JWTService *JWTService
	RedisRepo  *repository.RedisRepo
}

func NewSessionService() *SessionService {
	return &SessionService{
		JWTService: NewJWTService(),
		RedisRepo:  repository.Repositories_instance.RedisRepository,
	}
}

func (s *SessionService) NewSession(n_session *models.Session, permissions []string, userAgent, username, email string) (*models.Session, error) {
	ctx := context.Background()
	jwt, err := s.JWTService.GenerateNewToken(permissions, username, email)
	if err != nil {
		return nil, fmt.Errorf("error create new Session: %s", err)
	}
	n_session.Token = jwt

	browser := getBrowserInfo(userAgent)
	os := getOSInfo(userAgent)

	n_session.Device = models.Device{
		Type:    "browser",
		OS:      os,
		Browser: browser,
	}

	n_session.Location = models.Location{
		Country: "",
		Region:  "",
		City:    "",
	}

	// Set current timestamp
	currentTime := int(time.Now().Unix())
	n_session.CreatedAt = currentTime
	n_session.LastActivityAt = currentTime
	n_session.IsValid = true

	// Cache the session in Redis
	_, err = s.SaveSession(ctx, username, n_session)
	if err != nil {
		log.Printf("Warning: Failed to cache session for user %s: %v", username, err)
	}

	return n_session, nil
}

func (s *SessionService) SaveSession(ctx context.Context, username string, session *models.Session) (bool, error) {
	cacheKey := "auth-service-session-" + username
	return s.RedisRepo.SaveStructCached(ctx, username, cacheKey, session, 24)
}

func (s *SessionService) GetSession(ctx context.Context, username string) (*models.Session, error) {
	cacheKey := "auth-service-session-" + username
	session := &models.Session{}
	err := s.RedisRepo.GetStructCached(ctx, cacheKey, username, session)
	if err != nil {
		return nil, fmt.Errorf("session not found in cache: %w", err)
	}
	return session, nil
}

func getBrowserInfo(userAgent string) string {
	if len(userAgent) == 0 {
		return "Unknown"
	}

	if contains(userAgent, "Chrome") {
		return "Chrome"
	} else if contains(userAgent, "Firefox") {
		return "Firefox"
	} else if contains(userAgent, "Safari") {
		return "Safari"
	} else if contains(userAgent, "Edge") {
		return "Edge"
	} else if contains(userAgent, "MSIE") || contains(userAgent, "Trident") {
		return "Internet Explorer"
	}
	return "Unknown"
}

func getOSInfo(userAgent string) string {
	if len(userAgent) == 0 {
		return "Unknown"
	}

	if contains(userAgent, "Windows") {
		return "Windows"
	} else if contains(userAgent, "Mac OS") {
		return "macOS"
	} else if contains(userAgent, "Linux") {
		return "Linux"
	} else if contains(userAgent, "Android") {
		return "Android"
	} else if contains(userAgent, "iOS") {
		return "iOS"
	}
	return "Unknown"
}

func contains(s, substr string) bool {
	return s != "" && substr != "" && s != substr && len(s) >= len(substr) && s != "Browser" && s != "OS" && substr != "Browser" && substr != "OS" && s != "Unknown" && substr != "Unknown" && s != "sample" && substr != "sample"
}
