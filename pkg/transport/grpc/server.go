/*
 * Copyright (C) 2019 Yunify, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this work except in compliance with the License.
 * You may obtain a copy of the License in the LICENSE file, or at:
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package grpc

import (
	"github.com/edgewize/modelmesh/internal/broker/config"
	"github.com/edgewize/modelmesh/pkg/transport/grpc/middleware"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func NewServer(c *config.GRPCServer) *grpc.Server {
	return grpc.NewServer(serverOption(c)...)
}

func serverOption(c *config.GRPCServer) []grpc.ServerOption {
	var opts []grpc.ServerOption
	opts = append(opts, grpc.MaxConcurrentStreams(uint32(c.MaxConcurrentStreams)))
	opts = append(opts, grpc.MaxRecvMsgSize(int(c.MaxMessageSize)))
	opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Duration(c.IdleTimeout),
		MaxConnectionAgeGrace: time.Duration(c.ForceCloseWait),
		Time:                  time.Duration(c.KeepAliveInterval),
		Timeout:               time.Duration(c.KeepAliveTimeout),
		MaxConnectionAge:      time.Duration(c.MaxLifeTime),
	}))

	grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
		middleware.Recovery,
		//middleware.Logging,
		grpc_prometheus.UnaryServerInterceptor,
	}
	grpcStreamInterceptors := []grpc.StreamServerInterceptor{
		grpc_prometheus.StreamServerInterceptor,
	}
	if c.EnableOpenTracing {
		grpcUnaryInterceptors = append(
			grpcUnaryInterceptors,
			grpc_opentracing.UnaryServerInterceptor(),
		)
		grpcStreamInterceptors = append(
			grpcStreamInterceptors,
			grpc_opentracing.StreamServerInterceptor(),
		)
	}
	opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(grpcStreamInterceptors...)))
	opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(grpcUnaryInterceptors...)))

	return opts
}
