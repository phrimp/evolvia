package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Id          string
	UserID      string
	Username    string
	Email       string
	Permissions []string
}
