package service

import (
	"auth_service/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct{}

func NewJWTService() *JWTService {
	return &JWTService{}
}

func (jwt_s *JWTService) GenerateNewToken(user models.UserAuth) *jwt.Token {
	claim_id := "C-" + GenerateRandomStringWithLength(6)
	claim := models.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Issuer:   "auth-service",
		},
		Id:       claim_id,
		Username: user.Username,
		Email:    user.Email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	return token
}
