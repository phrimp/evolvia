package grpc

import (
	"auth_service/internal/models"
	"auth_service/pkg/discovery"
	"context"
	"log"
	"os"

	pb "auth_service/pkg/proto/auth"
	common "auth_service/pkg/proto/shared"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SessionSender struct {
	middlewareAddr string
}

func NewSessionSender() *SessionSender {
	middlewareAddr, err := discovery.ServiceDiscovery.GetServiceAddress(os.Getenv("MIDDLEWARE_SERVICE_NAME"))
	if err != nil {
		log.Printf("Find Middleware Address Failed: %s, Use Default Address: middleware:9000", err)
		if middlewareAddr == "" {
			middlewareAddr = "middleware:9000"
		}
	}

	return &SessionSender{
		middlewareAddr: middlewareAddr,
	}
}

func (s *SessionSender) SendSession(ctx context.Context, session *models.Session) error {
	conn, err := grpc.NewClient(s.middlewareAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to middleware: %v", err)
		return err
	}
	defer conn.Close()

	client := pb.NewAuthServiceClient(conn)

	sessionData := &pb.SessionData{
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
