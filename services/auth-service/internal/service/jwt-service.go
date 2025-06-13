package service

import (
	"auth_service/internal/config"
	"auth_service/internal/models"
	"fmt"
	utils "proto-gen/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct{}

func NewJWTService() *JWTService {
	return &JWTService{}
}

func (jwt_s *JWTService) GenerateNewToken(permissions []string, username, email, userID string) (string, error) {
	claim_id := "C-" + utils.GenerateRandomStringWithLength(6)
	claim := models.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer:   "auth-service",
		},
		Id:          claim_id,
		UserID:      userID,
		Username:    username,
		Email:       email,
		Permissions: permissions,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenString, err := token.SignedString([]byte(config.ServiceConfig.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("error generate token string: %s", err)
	}
	return tokenString, nil
}
