package xconfig

import (
	"time"
)

// RPCClient is RPC client config.
type GRPCClient struct {
	Addr    string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`
}

// RPCServer is RPC server config.
type GRPCServer struct {
	Addr                 string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	Timeout              time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`
	IdleTimeout          time.Duration `yaml:"idleTimeout,omitempty" json:"idleTimeout,omitempty" mapstructure:"idleTimeout"`
	MaxLifeTime          time.Duration `yaml:"maxLifeTime,omitempty" json:"maxLifeTime,omitempty" mapstructure:"maxLifeTime"`
	ForceCloseWait       time.Duration `yaml:"forceCloseWait,omitempty" json:"forceCloseWait,omitempty" mapstructure:"forceCloseWait"`
	KeepAliveInterval    time.Duration `yaml:"keepAliveInterval,omitempty" json:"keepAliveInterval,omitempty" mapstructure:"keepAliveInterval"`
	KeepAliveTimeout     time.Duration `yaml:"keepAliveTimeout,omitempty" json:"keepAliveTimeout,omitempty" mapstructure:"keepAliveTimeout"`
	MaxMessageSize       int32         `yaml:"maxMessageSize,omitempty" json:"maxMessageSize,omitempty" mapstructure:"maxMessageSize"`
	MaxConcurrentStreams int32         `yaml:"maxConcurrentStreams,omitempty" json:"maxConcurrentStreams,omitempty" mapstructure:"maxConcurrentStreams"`
	EnableOpenTracing    bool          `yaml:"enableOpenTracing,omitempty" json:"enableOpenTracing,omitempty" mapstructure:"enableOpenTracing"`
}

type BaseConfig struct {
	ProfPathPrefix string   `yaml:"profPathPrefix,omitempty" json:"profPathPrefix,omitempty" mapstructure:"profPathPrefix"`
	LogLevel       string   `yaml:"logLevel,omitempty" json:"logLevel,omitempty" mapstructure:"logLevel"`
	ProfEnable     bool     `yaml:"profEnable,omitempty" json:"profEnable,omitempty" mapstructure:"profEnable"`
	TracerEnable   bool     `yaml:"tracerEnable,omitempty" json:"tracerEnable,omitempty" mapstructure:"tracerEnable"`
	WhiteList      []string `yaml:"whiteList,omitempty" json:"whiteList,omitempty" mapstructure:"whiteList"`
	Endpoints      []string `yaml:"endpoints,omitempty" json:"endpoints,omitempty" mapstructure:"endpoints"`
	BaseConfig     string   `yaml:"baseConfig,omitempty" json:"baseConfig,omitempty" mapstructure:"baseConfig"`
}

// ServiceQueue 封装底层 ServingServer 并提供 ServiceQueue 抽象，一个 ServiceQueue 可以设置对应的调度策略
type Schedule struct {
	Method             string `yaml:"method,omitempty" json:"method,omitempty" mapstructure:"method"`
	DisableFlowControl bool   `yaml:"disableFlowControl,omitempty" json:"disableFlowControl,omitempty" mapstructure:"disableFlowControl"`
}

// ServiceGroup 表示一个服务组，服务组的资源分配基于权重，权重高的服务组可以获得更多的资源，具体调度策略由 ServiceQueue 的 SchedulingMethod 决定
type ServiceGroup struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	//reclaimable表示该queue在资源使用量超过该queue所应得的资源份额时，是否允许其他queue回收该queue使用超额的资源，默认值为true
	Reclaimable bool `yaml:"reclaimable,omitempty" json:"reclaimable,omitempty" mapstructure:"reclaimable"`
	//weight表示该ServiceQueue在集群资源划分中所占的相对比重
	//该ServiceQueue应得资源总量为 (weight/total-weight) * total-resource。
	//其中， total-weight表示所有的queue的weight总和，total-resource表示集群的资源总量。
	//weight是一个软约束，取值范围为[1, 2^31-1]
	Weight int32 `yaml:"weight,omitempty" json:"weight,omitempty" mapstructure:"weight"`
}

func (sg *ServiceGroup) ResourceName() string {
	return sg.Name
}

type Queue struct {
	Size int64 `yaml:"size,omitempty" json:"size,omitempty" mapstructure:"size"` // 服务队列大小
}

// ServiceQueue 封装底层 ServingServer 并提供 ServiceQueue 抽象，一个 ServiceQueue 可以设置对应的调度策略
type Dispatch struct {
	PoolSize int           `yaml:"poolSize,omitempty" json:"poolSize,omitempty" mapstructure:"poolSize"`
	Timeout  time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`
	Queue    *Queue        `yaml:"queue,omitempty" json:"queue,omitempty" mapstructure:"queue"` // 等待队列
	Client   *GRPCClient   `yaml:"client,omitempty" json:"client,omitempty" mapstructure:"client"`
}

type PromMetrics struct {
	Addr           string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	ScrapeInterval time.Duration `yaml:"scrapeInterval,omitempty" json:"scrapeInterval,omitempty" mapstructure:"scrapeInterval"`
}
