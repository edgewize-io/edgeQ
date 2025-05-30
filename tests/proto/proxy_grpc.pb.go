// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: proxy.proto

package proxy

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	BackendService_BidirectionalStream_FullMethodName = "/BackendService/BidirectionalStream"
)

// BackendServiceClient is the client API for BackendService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BackendServiceClient interface {
	BidirectionalStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[DataChunk, DataChunk], error)
}

type backendServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBackendServiceClient(cc grpc.ClientConnInterface) BackendServiceClient {
	return &backendServiceClient{cc}
}

func (c *backendServiceClient) BidirectionalStream(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[DataChunk, DataChunk], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &BackendService_ServiceDesc.Streams[0], BackendService_BidirectionalStream_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[DataChunk, DataChunk]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BackendService_BidirectionalStreamClient = grpc.BidiStreamingClient[DataChunk, DataChunk]

// BackendServiceServer is the server API for BackendService service.
// All implementations should embed UnimplementedBackendServiceServer
// for forward compatibility.
type BackendServiceServer interface {
	BidirectionalStream(grpc.BidiStreamingServer[DataChunk, DataChunk]) error
}

// UnimplementedBackendServiceServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedBackendServiceServer struct{}

func (UnimplementedBackendServiceServer) BidirectionalStream(grpc.BidiStreamingServer[DataChunk, DataChunk]) error {
	return status.Errorf(codes.Unimplemented, "method BidirectionalStream not implemented")
}
func (UnimplementedBackendServiceServer) testEmbeddedByValue() {}

// UnsafeBackendServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BackendServiceServer will
// result in compilation errors.
type UnsafeBackendServiceServer interface {
	mustEmbedUnimplementedBackendServiceServer()
}

func RegisterBackendServiceServer(s grpc.ServiceRegistrar, srv BackendServiceServer) {
	// If the following call pancis, it indicates UnimplementedBackendServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&BackendService_ServiceDesc, srv)
}

func _BackendService_BidirectionalStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BackendServiceServer).BidirectionalStream(&grpc.GenericServerStream[DataChunk, DataChunk]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type BackendService_BidirectionalStreamServer = grpc.BidiStreamingServer[DataChunk, DataChunk]

// BackendService_ServiceDesc is the grpc.ServiceDesc for BackendService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BackendService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "BackendService",
	HandlerType: (*BackendServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "BidirectionalStream",
			Handler:       _BackendService_BidirectionalStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proxy.proto",
}
