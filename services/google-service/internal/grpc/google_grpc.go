package grpc

import (
	"context"
	"google-service/internal/services"
	"log"
	"proto-gen/shared"
)

type GoogleServiceServer struct {
	oauthService *services.GoogleOAuthService
}

func NewGoogleServiceServer() *GoogleServiceServer {
	return &GoogleServiceServer{}
}

func (s *GoogleServiceServer) ProcessSession(ctx context.Context, req *shared.GoogleLoginData) (*shared.GoogleLoginResponse, error) {
	log.Printf("Received session google auth code: %s", req.Code)

	token, err := s.oauthService.Exchange(ctx, req.Code)
	if err != nil {
		log.Printf("Token exchange error: %v\n", err)
		return nil, err
	}

	userInfo, err := s.oauthService.GetUserInfo(token)
	if err != nil {
		return nil, err
	}

	type User struct {
		DisplayName string `json:"displayName"`
		AvatarUrl   string `json:"avatar_url"`
	}
	basic_profile := User{DisplayName: userInfo.Name, AvatarUrl: userInfo.Picture}

	return &shared.GoogleLoginResponse{DisplayName: basic_profile.DisplayName, AvatarUrl: basic_profile.AvatarUrl}, nil
}
