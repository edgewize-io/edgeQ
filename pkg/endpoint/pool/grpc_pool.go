package pool

import (
	"context"
	"errors"
	config "github.com/edgewize/edgeQ/pkg/config"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

var ErrPoolInit = errors.New("Pool init error")

type DialOptions struct {
	DialTimeout           time.Duration
	BackoffMaxDelay       time.Duration
	InitialWindowSize     int32
	InitialConnWindowSize int32
	MaxSendMsgSize        int
	MaxRecvMsgSize        int
	KeepAlive             time.Duration
	KeepAliveTimeout      time.Duration
	BackendAddr           string
}

type Pool struct {
	clients                 chan *Client
	name                    string        // 连接池名称
	proxyModel              string        // 连接池负载模式
	code                    string        // 连接池 code
	connCurrent             int32         // 当前连接数
	capacity                int32         // 容量
	weight                  int32         // 权重
	size                    int32         // 容量大小 (动态变化)
	idleDur                 time.Duration // 空闲时间
	maxLifeDur              time.Duration // 最大连接时间
	lock                    sync.RWMutex  // 读写锁
	poolRemoteAddr          string        // 远程连接地址
	averageRequestTimeTotal int64         // 耗时统计
	averageRequestTime      int64         // 平均耗时
	averageRequestTimeNum   int64         // 耗时计算数量单元
	sumRequestTimes         int64         // 总请求次数
	status                  bool          // 是否可用
	acquireConnTimeout      time.Duration
	dialOptions             DialOptions
}

type Client struct {
	*grpc.ClientConn
	timeUsed time.Time
	timeInit time.Time
	pool     *Pool
}

//type StreamDirector func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, *Client, error)

type StreamDirector interface {
	GetConn(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, *Client, error)
	GetScheduleMethod() string
}

func NewGrpcPool(grpcCfg config.GRPCConfig) (*Pool, error) {
	return initPool(grpcCfg)
}

func initPool(grpcCfg config.GRPCConfig) (*Pool, error) {
	if grpcCfg.ConnectionNum < 0 || grpcCfg.RequestIdleTime < 0 || grpcCfg.RequestMaxLife < 0 {
		return nil, ErrPoolInit
	}

	pool := &Pool{
		clients:            make(chan *Client, grpcCfg.ConnectionNum),
		size:               grpcCfg.ConnectionNum,
		capacity:           grpcCfg.ConnectionNum,
		idleDur:            grpcCfg.RequestIdleTime,
		maxLifeDur:         grpcCfg.RequestMaxLife,
		acquireConnTimeout: grpcCfg.AcquireConnTimeout,
		status:             true,
		dialOptions: DialOptions{
			DialTimeout:           grpcCfg.DialTimeout,
			BackoffMaxDelay:       grpcCfg.BackoffMaxDelay,
			InitialWindowSize:     grpcCfg.InitialWindowSize,
			InitialConnWindowSize: grpcCfg.InitialConnWindowSize,
			MaxSendMsgSize:        grpcCfg.MaxSendMsgSize,
			MaxRecvMsgSize:        grpcCfg.MaxRecvMsgSize,
			KeepAlive:             grpcCfg.KeepAlive,
			KeepAliveTimeout:      grpcCfg.KeepAliveTimeout,
			BackendAddr:           grpcCfg.BackendAddr,
		},
	}

	for i := int32(0); i < grpcCfg.ConnectionNum; i++ {
		client, err := pool.createClient()
		if err != nil {
			return nil, ErrPoolInit
		}
		pool.clients <- client
	}
	return pool, nil
}

func (p *Pool) createConn() (*grpc.ClientConn, error) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), p.dialOptions.DialTimeout)
	defer ctxCancel()
	gcc, err := grpc.DialContext(ctx,
		p.dialOptions.BackendAddr,
		grpc.WithCodec(Codec()),
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(p.dialOptions.BackoffMaxDelay),
		grpc.WithInitialWindowSize(p.dialOptions.InitialWindowSize),
		grpc.WithInitialConnWindowSize(p.dialOptions.InitialConnWindowSize),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(p.dialOptions.MaxSendMsgSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(p.dialOptions.MaxRecvMsgSize)),
		//grpc.WithKeepaliveParams(keepalive.ClientParameters{
		//	Time:                p.dialOptions.KeepAlive,
		//	Timeout:             p.dialOptions.KeepAliveTimeout,
		//	PermitWithoutStream: true,
		//}),
	)

	if err != nil {
		klog.Errorf("grpc dial failed, %v!", err)
	}

	return gcc, err
}

func (p *Pool) createClient() (*Client, error) {
	conn, err := p.createConn()
	if err != nil {
		return nil, ErrPoolInit
	}
	now := time.Now()
	client := &Client{
		ClientConn: conn,
		timeUsed:   now,
		timeInit:   now,
		pool:       p,
	}
	// atomic.AddInt32(&p.connCurrent, 1)
	return client, nil
}

// 从连接池取出一个连接
func (p *Pool) Acquire(ctx context.Context) (*Client, error) {
	if p.IsClose() {
		return nil, errors.New("Pool is closed")
	}

	// defer func() {
	// 	atomic.AddInt32(&p.connCurrent, 1)
	// }()

	var client *Client
	now := time.Now()
	select {
	case <-ctx.Done():
		klog.Info("ctx done after client close !")
		return nil, errors.New("ctx done after client close !")
	case client = <-p.clients:
		// per request time
		if client != nil && p.idleDur > 0 && client.timeUsed.Add(p.idleDur).After(now) {
			client.timeUsed = now
			return client, nil
		}
	case <-time.After(p.acquireConnTimeout):
		klog.Warning("acquire conn timeout!")
		return nil, errors.New("acquire conn timeout")
	}

	if client != nil {
		client.Destory()
	}

	client, err := p.createClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// 连接池关闭
func (p *Pool) Close() {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.IsClose() {
		return
	}

	clients := p.clients
	p.clients = nil

	// 异步处理池里的连接
	go func() {
		for len(clients) > 0 {
			client := <-clients
			if client != nil {
				client.Destory()
			}
		}
	}()

	p.status = false
}

// 连接池是否关闭
func (p *Pool) IsClose() bool {
	return p == nil || p.clients == nil
}

// 连接池中连接数
func (p *Pool) Size() int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return len(p.clients)
}

// 实际连接数
func (p *Pool) GetConnCurrent() int32 {
	return p.capacity - int32(p.Size())
}

// 连接关闭
func (client *Client) Close() {
	go func() {
		pool := client.pool
		now := time.Now()
		// 连接池关闭了直接销毁
		if pool.IsClose() {
			client.Destory()
			return
		}
		// 如果连接存活时间超长也直接销毁连接
		// if p.maxLifeDur > 0 && client.timeInit.Add(p.maxLifeDur).Before(now) {
		// 	client.Destory()
		// 	return
		// }

		if client.ClientConn == nil {
			return
		}
		client.timeUsed = now
		client.pool.clients <- client

	}()
}

// 销毁 client
func (client *Client) Destory() {
	if client.ClientConn != nil {
		client.ClientConn.Close()
		// atomic.AddInt32(&client.p.connCurrent, -1)
	}
	client.ClientConn = nil
	client.pool = nil
}

// 获取连接创建时间
func (client *Client) TimeInit() time.Time {
	return client.timeInit
}

// 获取连接上一次使用时间
func (client *Client) TimeUsed() time.Time {
	return client.timeUsed
}
