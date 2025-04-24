package grpc

import (
	"context"
	"log"
	"middleware/internal/models"
	pb "proto-gen/middleware"
	"proto-gen/shared"
)

type MiddlewareServer struct {
	pb.UnimplementedMiddlewareServiceServer
}

func NewMiddlewareServer() *MiddlewareServer {
	return &MiddlewareServer{}
}

func (s *MiddlewareServer) ProcessSession(ctx context.Context, req *shared.SessionData) (*shared.SessionResponse, error) {
	log.Printf("Received session data for user ID: %s", req.UserId)

	session := &models.Session{
		Token:          req.Token,
		UserAgent:      req.UserAgent,
		IPAddress:      req.IpAddress,
		IsValid:        req.IsValid,
		CreatedAt:      int(req.CreatedAt),
		LastActivityAt: int(req.LastActivityAt),
		Device: models.Device{
			Type:    req.Device.Type,
			OS:      req.Device.Os,
			Browser: req.Device.Browser,
		},
		Location: models.Location{
			Country: req.Location.Country,
			Region:  req.Location.Region,
			City:    req.Location.City,
		},
	}

	log.Printf("Processing session: %v", session)

	return &shared.SessionResponse{
		Success: true,
		Message: "Session processed successfully",
	}, nil
}
