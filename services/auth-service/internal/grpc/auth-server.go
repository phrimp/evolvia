package grpc

import (
	pb "proto-gen/auth"
)

type AuthServer struct {
	pb.UnimplementedAuthServiceServer
}

func NewAuthServer() *AuthServer {
	return &AuthServer{}
}
