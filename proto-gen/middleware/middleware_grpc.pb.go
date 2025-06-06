// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v6.31.0
// source: proto/middleware/middleware.proto

package middleware

import (
	context "context"
	shared "proto-gen/shared"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	MiddlewareService_ProcessSession_FullMethodName = "/middleware.MiddlewareService/ProcessSession"
)

// MiddlewareServiceClient is the client API for MiddlewareService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MiddlewareServiceClient interface {
	ProcessSession(ctx context.Context, in *shared.SessionData, opts ...grpc.CallOption) (*shared.SessionResponse, error)
}

type middlewareServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMiddlewareServiceClient(cc grpc.ClientConnInterface) MiddlewareServiceClient {
	return &middlewareServiceClient{cc}
}

func (c *middlewareServiceClient) ProcessSession(ctx context.Context, in *shared.SessionData, opts ...grpc.CallOption) (*shared.SessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(shared.SessionResponse)
	err := c.cc.Invoke(ctx, MiddlewareService_ProcessSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MiddlewareServiceServer is the server API for MiddlewareService service.
// All implementations must embed UnimplementedMiddlewareServiceServer
// for forward compatibility.
type MiddlewareServiceServer interface {
	ProcessSession(context.Context, *shared.SessionData) (*shared.SessionResponse, error)
	mustEmbedUnimplementedMiddlewareServiceServer()
}

// UnimplementedMiddlewareServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedMiddlewareServiceServer struct{}

func (UnimplementedMiddlewareServiceServer) ProcessSession(context.Context, *shared.SessionData) (*shared.SessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ProcessSession not implemented")
}
func (UnimplementedMiddlewareServiceServer) mustEmbedUnimplementedMiddlewareServiceServer() {}
func (UnimplementedMiddlewareServiceServer) testEmbeddedByValue()                           {}

// UnsafeMiddlewareServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MiddlewareServiceServer will
// result in compilation errors.
type UnsafeMiddlewareServiceServer interface {
	mustEmbedUnimplementedMiddlewareServiceServer()
}

func RegisterMiddlewareServiceServer(s grpc.ServiceRegistrar, srv MiddlewareServiceServer) {
	// If the following call pancis, it indicates UnimplementedMiddlewareServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&MiddlewareService_ServiceDesc, srv)
}

func _MiddlewareService_ProcessSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(shared.SessionData)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MiddlewareServiceServer).ProcessSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MiddlewareService_ProcessSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MiddlewareServiceServer).ProcessSession(ctx, req.(*shared.SessionData))
	}
	return interceptor(ctx, in, info, handler)
}

// MiddlewareService_ServiceDesc is the grpc.ServiceDesc for MiddlewareService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var MiddlewareService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "middleware.MiddlewareService",
	HandlerType: (*MiddlewareServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProcessSession",
			Handler:    _MiddlewareService_ProcessSession_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/middleware/middleware.proto",
}
