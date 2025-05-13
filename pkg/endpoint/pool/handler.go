package pool

import (
	"context"
	"fmt"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	"github.com/edgewize/edgeQ/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"io"
	"k8s.io/klog/v2"
	"time"
)

var (
	grpcRetryTimesInt           = 10
	grpcRetrySleepTimesInt      = 100
	clientStreamDescForProxying = &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}
)

type protoCodec struct{}

// codec
func Codec() grpc.Codec {
	return CodecWithParent(&protoCodec{})
}

// codec with parent
func CodecWithParent(fallback grpc.Codec) grpc.Codec {
	return &rawCodec{fallback}
}

// raw codec
type rawCodec struct {
	parentCodec grpc.Codec
}

// frame struct
type frame struct {
	payload []byte
}

// marshal
func (c *rawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.payload, nil

}

// unmarshal
func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.payload = data
	return nil
}

// string func
func (c *rawCodec) String() string {
	return fmt.Sprintf("proxy>%s", c.parentCodec.String())
}

// marshal
func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

// unmarshal
func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

// string func()
func (protoCodec) String() string {
	return "proto"
}

// register service
func RegisterService(server *grpc.Server, director StreamDirector, serviceName string, methodNames ...string) {
	streamer := &handler{
		director: director,
	}
	fakeDesc := &grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*interface{})(nil),
	}
	for _, m := range methodNames {
		streamDesc := grpc.StreamDesc{
			StreamName:    m,
			Handler:       streamer.handler,
			ServerStreams: true,
			ClientStreams: true,
		}
		fakeDesc.Streams = append(fakeDesc.Streams, streamDesc)
	}
	server.RegisterService(fakeDesc, streamer)
}

// transparent handler
func TransparentHandler(director StreamDirector) grpc.StreamHandler {
	streamer := &handler{director: director}
	return func(srv interface{}, stream grpc.ServerStream) error {
		startTime := time.Now()
		serviceGroup := utils.ServiceGroupFromContext(stream.Context())
		err := streamer.handler(srv, stream)
		metrics.BackendReceivedRequestTotalRecord(director.GetScheduleMethod(), serviceGroup, time.Now().Sub(startTime).Seconds())
		fmt.Printf("请求处理完成\n")
		return err
	}
}

// handler struct
type handler struct {
	director StreamDirector
}

// handler func
func (h *handler) handler(srv interface{}, serverStream grpc.ServerStream) error {
	now := time.Now()
	timeStart := now.UnixMilli()

	fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		return status.Errorf(codes.Internal, "lowLevelServerStream not exists in context")
	}

	outgoingCtx, backendConn, conn, err := h.director.GetConn(serverStream.Context(), fullMethodName)
	if err != nil {
		c := 0
		for ; c < grpcRetryTimesInt; c++ {
			outgoingCtx, backendConn, conn, err = h.director.GetConn(serverStream.Context(), fullMethodName)
			if err == nil {
				break
			}
		}
		if c >= grpcRetryTimesInt {
			klog.Errorf("-----------------------  create stream director: %v", err)
			return err
		}
	}

	defer conn.Close()

	clientCtx, clientCancel := context.WithCancel(outgoingCtx)
	defer clientCancel()
	// TODO(mwitkow): Add a `forwarded` header to metadata, https://en.wikipedia.org/wiki/X-Forwarded-For.
	clientStream, err := grpc.NewClientStream(clientCtx, clientStreamDescForProxying, backendConn, fullMethodName)
	if err != nil {
		c := 0
		for ; c < grpcRetryTimesInt; c++ {
			klog.Errorf("-----------------------  create stream error: %v", err)
			clientStream, err = grpc.NewClientStream(clientCtx, clientStreamDescForProxying, backendConn, fullMethodName)
			if err != nil {
				SleepTime(grpcRetrySleepTimesInt)
			} else {
				break
			}
		}
		// return err
		if c >= grpcRetryTimesInt {
			return err
		}
	}

	s2cErrChan := h.forwardServerToClient(serverStream, clientStream)
	c2sErrChan := h.forwardClientToServer(clientStream, serverStream)
	for i := 0; i < 2; i++ {
		select {
		case s2cErr := <-s2cErrChan:
			if s2cErr == io.EOF {
				clientStream.CloseSend()
				// break
			} else {
				clientCancel()
				klog.Errorf("-----------------------  create stream S2C: %v, failed proxying s2c: %v ", codes.Internal, s2cErr)
				return status.Errorf(codes.Internal, "failed proxying s2c: %v", s2cErr)
			}
		case c2sErr := <-c2sErrChan:
			endNow := time.Now()
			timeEnd := endNow.UnixMilli()
			klog.Infof("c 2 s from %v to %v, request time spend: %v", timeStart, timeEnd, timeEnd-timeStart)
			serverStream.SetTrailer(clientStream.Trailer())
			if c2sErr != io.EOF {
				klog.Errorf("-----------------------  create stream S2C: %v,  failed proxying s2c: %v", codes.Internal, c2sErr)
				return c2sErr
			}
			return nil
		}
	}

	return status.Errorf(codes.Internal, "gRPC proxying should never reach this stage.")
}

// forward client to server
func (h *handler) forwardClientToServer(src grpc.ClientStream, dst grpc.ServerStream) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &frame{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				if err == io.EOF {
					ret <- err
					break
				}
				c := 0
				for ; c < grpcRetryTimesInt; c++ {
					err = src.RecvMsg(f)
					if err != nil {
						SleepTime(grpcRetrySleepTimesInt)
					} else {
						break
					}
				}
				if c >= grpcRetryTimesInt {
					ret <- err
					break
				}
			}
			if i == 0 {
				md, err := src.Header()
				if err != nil {
					ret <- err
					break
				}
				if err := dst.SendHeader(md); err != nil {
					ret <- err
					break
				}
			}
			if err := dst.SendMsg(f); err != nil {
				c := 0
				for ; c < grpcRetryTimesInt; c++ {
					err = dst.SendMsg(f)
					if err != nil {
						SleepTime(grpcRetrySleepTimesInt)
					} else {
						break
					}
				}
				if c >= grpcRetryTimesInt {
					ret <- err
					break
				}
			}
		}
	}()
	return ret
}

// forward server to client
func (h *handler) forwardServerToClient(src grpc.ServerStream, dst grpc.ClientStream) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &frame{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				if err == io.EOF {
					ret <- err
					break
				}
				c := 0
				for ; c < grpcRetryTimesInt; c++ {
					err = src.RecvMsg(f)
					if err != nil {
						SleepTime(grpcRetrySleepTimesInt)
					} else {
						break
					}
				}
				if c >= grpcRetryTimesInt {
					ret <- err
					break
				}
			}
			if err := dst.SendMsg(f); err != nil {
				c := 0
				for ; c < grpcRetryTimesInt; c++ {
					err = dst.SendMsg(f)
					if err != nil {
						SleepTime(grpcRetrySleepTimesInt)
					} else {
						break
					}
				}
				if c >= grpcRetryTimesInt {
					ret <- err
					break
				}
			}
		}
	}()
	return ret
}

// sleep times
func SleepTime(sleepTimes int) {
	for i := 0; i < sleepTimes/100; i++ {
		time.Sleep(time.Millisecond * 100)
	}
}
