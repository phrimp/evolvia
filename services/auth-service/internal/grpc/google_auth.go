package grpc

import (
	"auth_service/internal/models"
	"auth_service/pkg/discovery"
	"context"
	"fmt"
	"log"
	pb "proto-gen/google"

	common "proto-gen/shared"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GoogleAuthService struct {
	discoveryService *discovery.ServiceRegistry
}

func NewGoogleAuthService(discoveryService *discovery.ServiceRegistry) *GoogleAuthService {
	return &GoogleAuthService{
		discoveryService: discoveryService,
	}
}

func (s *GoogleAuthService) SendGoogleCallBackCode(ctx context.Context, serviceName, google_code string) (*models.UserProfile, string, error) {
	serviceAddr, err := s.discoveryService.GetServiceAddress(serviceName, "grpc")
	if err != nil {
		return nil, "", fmt.Errorf("failed to find service address: %v", err)
	}

	conn, err := grpc.NewClient(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to middleware: %v", err)
		return nil, "", err
	}
	defer conn.Close()

	client := pb.NewGoogleServiceClient(conn)
	data := &common.GoogleLoginData{Code: google_code}

	response, err := client.ProcessGoogleAuth(ctx, data)
	if err != nil {
		log.Printf("Error sending session: %v", err)
		return nil, "", err
	}

	profile := &models.UserProfile{
		DisplayName: response.DisplayName,
	}

	return profile, response.AvatarUrl, nil
}
