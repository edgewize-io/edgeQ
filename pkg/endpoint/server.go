package endpoint

import (
	grpcPool "github.com/edgewize/edgeQ/pkg/endpoint/pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
	"net"
)

func RunGRPCServer(stopChan chan struct{}, address string, director grpcPool.StreamDirector) (err error) {
	klog.Infof("Starting GRPC server on %s", address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		klog.Errorf("failed to listen: %v", err)
		return
	}
	// grpc new server
	srv := grpc.NewServer(grpc.CustomCodec(grpcPool.Codec()), grpc.UnknownServiceHandler(grpcPool.TransparentHandler(director)))
	// register service
	grpcPool.RegisterService(srv, director,
		"PingEmpty",
		"Ping",
		"PingError",
		"PingList",
	)
	reflection.Register(srv)

	go func() {
		select {
		case <-stopChan:
			srv.GracefulStop()
			klog.Infof("GRPC server stopped")
		}
	}()

	// start ser listen
	err = srv.Serve(lis)
	if err != nil {
		klog.Errorf("failed to serve: %v", err)
	}

	return
}
