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
	"context"
	"fmt"
	v1 "github.com/edgewize/modelmesh/api/modelfulx/v1alpha"
	"github.com/edgewize/modelmesh/internal/broker/config"
	proto "github.com/edgewize/modelmesh/mindspore_serving/proto"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
	"time"
)

func clientOption(conf *config.GRPCClient) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			grpc_opentracing.UnaryClientInterceptor(),
		),
		//grpc.WithTimeout(timeout),
		//grpc.WithConnectParams(grpc.ConnectParams{
		//	Backoff: backoff.Config{
		//		MaxDelay: time.Second * 3,
		//	},
		//}),
		grpc.WithInitialWindowSize(1 << 30),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64 * 1024 * 1024)),
	}
}

func GetGrpcConn(conf *config.GRPCClient) (conn *grpc.ClientConn, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	conn, err = grpc.DialContext(ctx, conf.Addr, clientOption(conf)...)
	return
}

func NewServingClient(conf *config.GRPCClient) proto.MSServiceClient {
	var conn *grpc.ClientConn
	var err error
	for attempt := 1; attempt <= 50; attempt++ {
		conn, err = GetGrpcConn(conf)
		if err != nil {
			klog.Warningf("connect failed, %v, attempt [%d]", err, attempt)
		} else {
			err = nil
			break
		}
	}

	if err != nil {
		err = fmt.Errorf("connect fail, endpoint: %s, err %w", conf.Addr, err)
		klog.Error(err)
		return nil
	}
	return proto.NewMSServiceClient(conn)
}

func NewBrokerClient(conf *config.GRPCClient) v1.MFServiceClient {
	var conn *grpc.ClientConn
	var err error
	for attempt := 1; attempt <= 50; attempt++ {
		conn, err = GetGrpcConn(conf)
		if err != nil {
			klog.Warningf("connect failed, %v, attempt [%d]", err, attempt)
		} else {
			err = nil
			break
		}
	}

	if err != nil {
		err = fmt.Errorf("connect fail, endpoint: %s, err %w", conf.Addr, err)
		klog.Error(err)
		return nil
	}

	return v1.NewMFServiceClient(conn)
}
