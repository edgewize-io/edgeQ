// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: api/modelfulx/v1alpha/modelmesh_service.proto

package v1

import (
	context "context"
	fmt "fmt"
	proto1 "github.com/edgewize/edgeQ/mindspore_serving/proto"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type PredictRequest struct {
	Id                   string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Mindspore            *proto1.PredictRequest `protobuf:"bytes,2,opt,name=mindspore,proto3" json:"mindspore,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *PredictRequest) Reset()         { *m = PredictRequest{} }
func (m *PredictRequest) String() string { return proto.CompactTextString(m) }
func (*PredictRequest) ProtoMessage()    {}
func (*PredictRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_f274e740733af38a, []int{0}
}
func (m *PredictRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PredictRequest.Unmarshal(m, b)
}
func (m *PredictRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PredictRequest.Marshal(b, m, deterministic)
}
func (m *PredictRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PredictRequest.Merge(m, src)
}
func (m *PredictRequest) XXX_Size() int {
	return xxx_messageInfo_PredictRequest.Size(m)
}
func (m *PredictRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PredictRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PredictRequest proto.InternalMessageInfo

func (m *PredictRequest) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PredictRequest) GetMindspore() *proto1.PredictRequest {
	if m != nil {
		return m.Mindspore
	}
	return nil
}

type PredictReply struct {
	Id                   string               `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Mindspore            *proto1.PredictReply `protobuf:"bytes,2,opt,name=mindspore,proto3" json:"mindspore,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *PredictReply) Reset()         { *m = PredictReply{} }
func (m *PredictReply) String() string { return proto.CompactTextString(m) }
func (*PredictReply) ProtoMessage()    {}
func (*PredictReply) Descriptor() ([]byte, []int) {
	return fileDescriptor_f274e740733af38a, []int{1}
}
func (m *PredictReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PredictReply.Unmarshal(m, b)
}
func (m *PredictReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PredictReply.Marshal(b, m, deterministic)
}
func (m *PredictReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PredictReply.Merge(m, src)
}
func (m *PredictReply) XXX_Size() int {
	return xxx_messageInfo_PredictReply.Size(m)
}
func (m *PredictReply) XXX_DiscardUnknown() {
	xxx_messageInfo_PredictReply.DiscardUnknown(m)
}

var xxx_messageInfo_PredictReply proto.InternalMessageInfo

func (m *PredictReply) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PredictReply) GetMindspore() *proto1.PredictReply {
	if m != nil {
		return m.Mindspore
	}
	return nil
}

func init() {
	proto.RegisterType((*PredictRequest)(nil), "modelmesh.PredictRequest")
	proto.RegisterType((*PredictReply)(nil), "modelmesh.PredictReply")
}

func init() {
	proto.RegisterFile("api/modelfulx/v1alpha/modelmesh_service.proto", fileDescriptor_f274e740733af38a)
}

var fileDescriptor_f274e740733af38a = []byte{
	// 227 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xd2, 0x4d, 0x2c, 0xc8, 0xd4,
	0xcf, 0xcd, 0x4f, 0x49, 0xcd, 0x49, 0x2b, 0xcd, 0xa9, 0xd0, 0x2f, 0x33, 0x4c, 0xcc, 0x29, 0xc8,
	0x48, 0x84, 0x88, 0xe4, 0xa6, 0x16, 0x67, 0xc4, 0x17, 0xa7, 0x16, 0x95, 0x65, 0x26, 0xa7, 0xea,
	0x15, 0x14, 0xe5, 0x97, 0xe4, 0x0b, 0x71, 0xc2, 0x25, 0xa4, 0x34, 0x72, 0x33, 0xf3, 0x52, 0x8a,
	0x0b, 0xf2, 0x8b, 0x52, 0x21, 0x6a, 0xf2, 0xd2, 0xf5, 0xc1, 0x6a, 0xf4, 0x73, 0x8b, 0x51, 0x35,
	0x29, 0xa5, 0x73, 0xf1, 0x05, 0x14, 0xa5, 0xa6, 0x64, 0x26, 0x97, 0x04, 0xa5, 0x16, 0x96, 0xa6,
	0x16, 0x97, 0x08, 0xf1, 0x71, 0x31, 0x65, 0xa6, 0x48, 0x30, 0x2a, 0x30, 0x6a, 0x70, 0x06, 0x31,
	0x65, 0xa6, 0x08, 0xb9, 0x72, 0x71, 0xc2, 0x4d, 0x93, 0x60, 0x52, 0x60, 0xd4, 0xe0, 0x36, 0x52,
	0xd7, 0x83, 0x8b, 0xe8, 0x41, 0xcd, 0x87, 0x18, 0xa7, 0x87, 0x6a, 0x56, 0x10, 0x42, 0xa7, 0x52,
	0x32, 0x17, 0x0f, 0x5c, 0xb2, 0x20, 0xa7, 0x12, 0xc3, 0x1a, 0x67, 0x4c, 0x6b, 0x54, 0x09, 0x5b,
	0x53, 0x90, 0x53, 0x89, 0x64, 0x89, 0x51, 0x00, 0x17, 0xa7, 0xaf, 0x5b, 0x30, 0xc4, 0x83, 0x42,
	0xce, 0x5c, 0xec, 0x50, 0x75, 0x42, 0x92, 0x7a, 0xf0, 0xb0, 0x41, 0x73, 0xa2, 0x94, 0x38, 0x36,
	0xa9, 0x82, 0x9c, 0x4a, 0x25, 0x06, 0x0d, 0x46, 0x03, 0x46, 0x27, 0x96, 0x28, 0xa6, 0x32, 0xc3,
	0x24, 0x36, 0xb0, 0xb5, 0xc6, 0x80, 0x00, 0x00, 0x00, 0xff, 0xff, 0xd3, 0x08, 0xf0, 0x53, 0x92,
	0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// MFServiceClient is the client API for MFService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MFServiceClient interface {
	Predict(ctx context.Context, opts ...grpc.CallOption) (MFService_PredictClient, error)
}

type mFServiceClient struct {
	cc *grpc.ClientConn
}

func NewMFServiceClient(cc *grpc.ClientConn) MFServiceClient {
	return &mFServiceClient{cc}
}

func (c *mFServiceClient) Predict(ctx context.Context, opts ...grpc.CallOption) (MFService_PredictClient, error) {
	stream, err := c.cc.NewStream(ctx, &_MFService_serviceDesc.Streams[0], "/modelmesh.MFService/Predict", opts...)
	if err != nil {
		return nil, err
	}
	x := &mFServicePredictClient{stream}
	return x, nil
}

type MFService_PredictClient interface {
	Send(*PredictRequest) error
	Recv() (*PredictReply, error)
	grpc.ClientStream
}

type mFServicePredictClient struct {
	grpc.ClientStream
}

func (x *mFServicePredictClient) Send(m *PredictRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *mFServicePredictClient) Recv() (*PredictReply, error) {
	m := new(PredictReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// MFServiceServer is the server API for MFService service.
type MFServiceServer interface {
	Predict(MFService_PredictServer) error
}

func RegisterMFServiceServer(s *grpc.Server, srv MFServiceServer) {
	s.RegisterService(&_MFService_serviceDesc, srv)
}

func _MFService_Predict_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MFServiceServer).Predict(&mFServicePredictServer{stream})
}

type MFService_PredictServer interface {
	Send(*PredictReply) error
	Recv() (*PredictRequest, error)
	grpc.ServerStream
}

type mFServicePredictServer struct {
	grpc.ServerStream
}

func (x *mFServicePredictServer) Send(m *PredictReply) error {
	return x.ServerStream.SendMsg(m)
}

func (x *mFServicePredictServer) Recv() (*PredictRequest, error) {
	m := new(PredictRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _MFService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "modelmesh.MFService",
	HandlerType: (*MFServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Predict",
			Handler:       _MFService_Predict_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/modelfulx/v1alpha/modelmesh_service.proto",
}
