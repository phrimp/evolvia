package grpc

import (
	"context"
	"fmt"
	"log"
	"middleware/internal/models"
	"middleware/internal/repository"
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
		ID:             req.SessionId,
		UserID:         req.UserId,
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

	if session.IPAddress == "invalidate" {
		err := repository.Redis_repo.DeleteKey(ctx, req.Token)
		if err != nil {
			log.Printf("error invalidate token: %s/%s", req.Token, err)
			return nil, err
		}
		log.Printf("invalidated token: %s", req.Token)

		return &shared.SessionResponse{
			Success: true,
			Message: "Session processed successfully",
		}, nil

	}
	_, err := repository.Redis_repo.SaveStructCached(ctx, req.Token, session, 24)
	if err != nil {
		err = fmt.Errorf("error saving session to cache: %s", err)
		return nil, err
	}

	log.Printf("Processing session: %v", session)

	return &shared.SessionResponse{
		Success: true,
		Message: "Session processed successfully",
	}, nil
}
