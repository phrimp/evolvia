package grpc

import (
	"auth_service/internal/models"
	"auth_service/pkg/discovery"
	"context"
	"fmt"
	"log"

	pb "auth_service/pkg/proto/auth"
	common "auth_service/pkg/proto/shared"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SessionSenderService struct {
	discoveryService *discovery.ServiceRegistry
}

func NewSessionSenderService(discoveryService *discovery.ServiceRegistry) *SessionSenderService {
	return &SessionSenderService{
		discoveryService: discoveryService,
	}
}

func (s *SessionSenderService) SendSession(ctx context.Context, session *models.Session, serviceName string) error {
	serviceAddr, err := s.discoveryService.GetServiceAddress(serviceName, "grpc")
	if err != nil {
		return fmt.Errorf("failed to find service address: %v", err)
	}

	conn, err := grpc.NewClient(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to middleware: %v", err)
		return err
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)

	sessionData := &common.SessionData{
		SessionId:      session.ID.Hex(),
		UserId:         session.UserID.Hex(),
		Token:          session.Token,
		UserAgent:      session.UserAgent,
		IpAddress:      session.IPAddress,
		IsValid:        session.IsValid,
		CreatedAt:      int64(session.CreatedAt),
		LastActivityAt: int64(session.LastActivityAt),
		Device: &common.Device{
			Type:    session.Device.Type,
			Os:      session.Device.OS,
			Browser: session.Device.Browser,
		},
		Location: &common.Location{
			Country: session.Location.Country,
			Region:  session.Location.Region,
			City:    session.Location.City,
		},
	}

	response, err := client.SendSessionToMiddleware(ctx, sessionData)
	if err != nil {
		log.Printf("Error sending session: %v", err)
		return err
	}

	log.Printf("Session sent successfully: %v", response.Message)
	return nil
}
