package grpc

import (
	"context"
	"fmt"
	proxyCfg "github.com/edgewize/edgeQ/internal/proxy/config"
	"github.com/edgewize/edgeQ/pkg/constants"
	ge "github.com/edgewize/edgeQ/pkg/endpoint"
	grpcPool "github.com/edgewize/edgeQ/pkg/endpoint/pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

type GRPCProxyServer struct {
	Addr             string
	StopChan         chan struct{}
	Pool             *grpcPool.Pool
	ServiceGroupName string
}

func NewGRPCProxyServer(cfg *proxyCfg.Config) (*GRPCProxyServer, error) {
	pool, err := grpcPool.NewGrpcPool(cfg.Proxy.GRPC)
	if err != nil {
		return nil, err
	}

	sgName := cfg.ServiceGroup.Name
	if sgName == "" {
		sgName = constants.DefaultServiceGroup
	}

	newServer := &GRPCProxyServer{
		Addr:             cfg.Proxy.Addr,
		Pool:             pool,
		StopChan:         make(chan struct{}),
		ServiceGroupName: sgName,
	}

	return newServer, nil
}

func (ps *GRPCProxyServer) ContextWithServiceGroup(ctx context.Context) context.Context {
	md := metadata.Pairs(constants.ServiceGroupContext, ps.ServiceGroupName)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx
}

func (ps *GRPCProxyServer) GetConn(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, *grpcPool.Client, error) {
	var err error
	var conn *grpcPool.Client
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Errorf("failed to get metadata")
	}

	fmt.Printf("收到请求\n")
	outCtx := ps.ContextWithServiceGroup(ctx)
	outCtx = metadata.NewOutgoingContext(ctx, md.Copy())
	conn, err = ps.Pool.Acquire(ctx)
	// conn not nil
	if conn != nil {
		return outCtx, conn.ClientConn, conn, err
	}
	// return unknow error
	return nil, nil, nil, status.Errorf(codes.Unimplemented, "unknown method")
}

func (ps *GRPCProxyServer) Start(_ context.Context) error {
	return ge.RunGRPCServer(ps.StopChan, ps.Addr, ps)
}

func (g *GRPCProxyServer) Stop() {
	close(g.StopChan)
}

func (ps *GRPCProxyServer) GetScheduleMethod() string {
	return ""
}
