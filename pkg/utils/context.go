package utils

import (
	"context"
	"google.golang.org/grpc/metadata"
	"k8s.io/klog/v2"
)

const (
	ServiceGroupTag = "x-service-group"
)

func ContextWithServiceGroup(ctx context.Context, ServiceGroup string) context.Context {
	md := metadata.Pairs(ServiceGroupTag, ServiceGroup)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx
}

func ServiceGroupFromContext(ctx context.Context) string {
	// Read metadata from client.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Errorf("failed to get metadata")
	}

	serviceGroup := "default"
	t, ok := md[ServiceGroupTag]
	if ok {
		klog.V(0).Infof("serviceGroup from metadata:")
		for i, e := range t {
			klog.V(0).Infof("%d:%v", i, e)
			serviceGroup = e
		}
	}

	return serviceGroup
}
