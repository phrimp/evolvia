package grpc

import (
	pb "auth_service/pkg/proto/auth"
)

type AuthServer struct {
	pb.UnimplementedAuthServiceServer
}

func NewAuthServer() *AuthServer {
	return &AuthServer{}
}
