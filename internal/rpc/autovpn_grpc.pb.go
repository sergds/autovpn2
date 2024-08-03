// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v5.27.1
// source: internal/rpc/autovpn.proto

package rpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// AutoVPNClient is the client API for AutoVPN service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AutoVPNClient interface {
	ExecuteTask(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (AutoVPN_ExecuteTaskClient, error)
}

type autoVPNClient struct {
	cc grpc.ClientConnInterface
}

func NewAutoVPNClient(cc grpc.ClientConnInterface) AutoVPNClient {
	return &autoVPNClient{cc}
}

func (c *autoVPNClient) ExecuteTask(ctx context.Context, in *ExecuteRequest, opts ...grpc.CallOption) (AutoVPN_ExecuteTaskClient, error) {
	stream, err := c.cc.NewStream(ctx, &AutoVPN_ServiceDesc.Streams[0], "/AutoVPN/ExecuteTask", opts...)
	if err != nil {
		return nil, err
	}
	x := &autoVPNExecuteTaskClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type AutoVPN_ExecuteTaskClient interface {
	Recv() (*ExecuteUpdate, error)
	grpc.ClientStream
}

type autoVPNExecuteTaskClient struct {
	grpc.ClientStream
}

func (x *autoVPNExecuteTaskClient) Recv() (*ExecuteUpdate, error) {
	m := new(ExecuteUpdate)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AutoVPNServer is the server API for AutoVPN service.
// All implementations must embed UnimplementedAutoVPNServer
// for forward compatibility
type AutoVPNServer interface {
	ExecuteTask(*ExecuteRequest, AutoVPN_ExecuteTaskServer) error
	mustEmbedUnimplementedAutoVPNServer()
}

// UnimplementedAutoVPNServer must be embedded to have forward compatible implementations.
type UnimplementedAutoVPNServer struct {
}

func (UnimplementedAutoVPNServer) ExecuteTask(*ExecuteRequest, AutoVPN_ExecuteTaskServer) error {
	return status.Errorf(codes.Unimplemented, "method ExecuteTask not implemented")
}
func (UnimplementedAutoVPNServer) mustEmbedUnimplementedAutoVPNServer() {}

// UnsafeAutoVPNServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AutoVPNServer will
// result in compilation errors.
type UnsafeAutoVPNServer interface {
	mustEmbedUnimplementedAutoVPNServer()
}

func RegisterAutoVPNServer(s grpc.ServiceRegistrar, srv AutoVPNServer) {
	s.RegisterService(&AutoVPN_ServiceDesc, srv)
}

func _AutoVPN_ExecuteTask_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ExecuteRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AutoVPNServer).ExecuteTask(m, &autoVPNExecuteTaskServer{stream})
}

type AutoVPN_ExecuteTaskServer interface {
	Send(*ExecuteUpdate) error
	grpc.ServerStream
}

type autoVPNExecuteTaskServer struct {
	grpc.ServerStream
}

func (x *autoVPNExecuteTaskServer) Send(m *ExecuteUpdate) error {
	return x.ServerStream.SendMsg(m)
}

// AutoVPN_ServiceDesc is the grpc.ServiceDesc for AutoVPN service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AutoVPN_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "AutoVPN",
	HandlerType: (*AutoVPNServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ExecuteTask",
			Handler:       _AutoVPN_ExecuteTask_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/rpc/autovpn.proto",
}
