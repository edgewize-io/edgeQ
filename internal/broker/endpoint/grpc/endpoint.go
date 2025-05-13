package grpc

import (
	"context"
	"fmt"
	brokerCfg "github.com/edgewize/edgeQ/internal/broker/config"
	"github.com/edgewize/edgeQ/internal/broker/metrics"
	sch "github.com/edgewize/edgeQ/internal/broker/scheduler"
	"github.com/edgewize/edgeQ/pkg/constants"
	ge "github.com/edgewize/edgeQ/pkg/endpoint"
	grpcPool "github.com/edgewize/edgeQ/pkg/endpoint/pool"
	"github.com/edgewize/edgeQ/pkg/queue"
	"github.com/edgewize/edgeQ/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"time"
)

type GRPCBrokerServer struct {
	Addr              string
	StopChan          chan struct{}
	Pool              *grpcPool.Pool
	QueuePool         *queue.QueuePool
	Scheduler         *sch.Scheduler
	ScheduleMethod    string
	WorkPool          chan struct{}
	EnableFlowControl bool
}

func NewGRPCBrokerServer(cfg *brokerCfg.Config) (*GRPCBrokerServer, error) {
	pool, err := grpcPool.NewGrpcPool(cfg.Broker.GRPC)
	if err != nil {
		return nil, err
	}

	queuePool := queue.NewQueuePool(cfg.Queue, cfg.ServiceGroups)

	newScheduler, err := sch.NewScheduler(cfg.Schedule)
	if err != nil {
		return nil, err
	}

	newScheduler.RegeneratePicker(cfg.ServiceGroups)

	newServer := &GRPCBrokerServer{
		Addr:              cfg.Broker.Addr,
		Pool:              pool,
		StopChan:          make(chan struct{}),
		QueuePool:         queuePool,
		Scheduler:         newScheduler,
		ScheduleMethod:    cfg.Schedule.Method,
		WorkPool:          make(chan struct{}, cfg.Broker.WorkPoolSize),
		EnableFlowControl: cfg.Schedule.EnableFlowControl,
	}

	return newServer, nil
}

func (bs *GRPCBrokerServer) ServiceGroupFromContext(ctx context.Context) string {
	// Read metadata from client.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Errorf("failed to get metadata")
	}

	serviceGroup := "default"
	t, ok := md[constants.ServiceGroupContext]
	if ok {
		klog.V(0).Infof("serviceGroup from metadata:")
		for i, e := range t {
			klog.V(0).Infof("%d:%v", i, e)
			serviceGroup = e
		}
	}

	return serviceGroup
}

func (bs *GRPCBrokerServer) GetConn(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, *grpcPool.Client, error) {
	var err error
	var conn *grpcPool.Client
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		klog.Errorf("failed to get metadata")
	}

	fmt.Printf("收到请求\n")

	if bs.enableFlowControl() {
		serviceGroup := utils.ServiceGroupFromContext(ctx)
		waitChan, err := bs.EnqueueRequests(serviceGroup)
		if err != nil {
			fmt.Printf("请求入队报错，%v\n", err)
			return nil, nil, nil, err
		}

		<-waitChan
	}

	outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
	conn, err = bs.Pool.Acquire(ctx)
	// conn not nil
	if conn != nil {
		return outCtx, conn.ClientConn, conn, err
	}
	// return unknow error
	return nil, nil, nil, status.Errorf(codes.Unimplemented, "unknown method")
}

func (bs *GRPCBrokerServer) EnqueueRequests(serviceGroup string) (waitChan chan struct{}, err error) {
	waitChan = make(chan struct{})
	reqItem := &queue.GRPCItem{
		ServiceGroup: serviceGroup,
		CreateTime:   time.Now(),
		WaitChan:     waitChan,
	}

	err = bs.QueuePool.Push(serviceGroup, reqItem)
	if err != nil {
		return
	}

	metrics.BrokerReceivedRequestTotalRecord(bs.GetScheduleMethod(), serviceGroup)
	return
}

func (bs *GRPCBrokerServer) ProcessRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-bs.StopChan:
			return
		default:
		}

		item, err := bs.QueuePool.Pop(bs.Scheduler)
		if err != nil {
			continue
		}

		if item == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		grpcItem, ok := item.(*queue.GRPCItem)
		if !ok {
			continue
		}

		metrics.BrokerProcessingDurationRecord(bs.ScheduleMethod, grpcItem.ResourceName(), time.Since(grpcItem.GetCreateTime()).Seconds())

		select {
		case <-ctx.Done():
			return
		case <-bs.StopChan:
			return
		case bs.WorkPool <- struct{}{}:
			bs.HandleRequest(grpcItem)
		}
	}
}

func (bs *GRPCBrokerServer) HandleRequest(grpcItem *queue.GRPCItem) {
	defer func() { <-bs.WorkPool }()

	close(grpcItem.WaitChan)
}

func (bs *GRPCBrokerServer) Start(ctx context.Context) error {
	go bs.ProcessRequests(ctx)
	return ge.RunGRPCServer(bs.StopChan, bs.Addr, bs)
}

func (bs *GRPCBrokerServer) Stop() {
	close(bs.StopChan)
}

func (bs *GRPCBrokerServer) enableFlowControl() bool {
	return bs.EnableFlowControl
}

func (bs *GRPCBrokerServer) GetScheduleMethod() string {
	return bs.ScheduleMethod
}
