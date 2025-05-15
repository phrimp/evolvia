package services

import (
	"fmt"
	"middleware/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secretKey []byte
}

func NewJWTService(jwtSecret string) *JWTService {
	fmt.Printf("DEBUG: JWT SECRET: %s", jwtSecret)
	return &JWTService{
		secretKey: []byte(jwtSecret),
	}
}

func (s *JWTService) VerifyToken(tokenString string) (*models.Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&models.Claims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return s.secretKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*models.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
