package xconfig

import (
	"time"
)

type GRPCClient struct {
	Addr    string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`
}

type MeshProxy struct {
	Addr            string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	DialTimeout     time.Duration `yaml:"dialTimeout,omitempty" json:"dialTimeout,omitempty" mapstructure:"dialTimeout"`
	KeepAlive       time.Duration `yaml:"keepAlive,omitempty" json:"keepAlive,omitempty" mapstructure:"keepAlive"`
	HeaderTimeout   time.Duration `yaml:"headerTimeout,omitempty" json:"headerTimeout,omitempty" mapstructure:"headerTimeout"`
	IdleConnTimeout time.Duration `yaml:"idleConnTimeout,omitempty" json:"idleConnTimeout,omitempty" mapstructure:"idleConnTimeout"`
	MaxIdleConns    int           `yaml:"maxIdleConns,omitempty" json:"maxIdleConns,omitempty" mapstructure:"maxIdleConns"`
	MaxConnsPerHost int           `yaml:"maxConnsPerHost,omitempty" json:"maxConnsPerHost,omitempty" mapstructure:"maxConnsPerHost"`
	ReadTimeout     time.Duration `yaml:"readTimeout,omitempty" json:"readTimeout,omitempty" mapstructure:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout,omitempty" json:"writeTimeout,omitempty" mapstructure:"writeTimeout"`
}

// RPCServer is RPC server config.
type MeshServer struct {
	Addr            string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	DialTimeout     time.Duration `yaml:"dialTimeout,omitempty" json:"dialTimeout,omitempty" mapstructure:"dialTimeout"`
	KeepAlive       time.Duration `yaml:"keepAlive,omitempty" json:"keepAlive,omitempty" mapstructure:"keepAlive"`
	HeaderTimeout   time.Duration `yaml:"headerTimeout,omitempty" json:"headerTimeout,omitempty" mapstructure:"headerTimeout"`
	IdleConnTimeout time.Duration `yaml:"idleConnTimeout,omitempty" json:"idleConnTimeout,omitempty" mapstructure:"idleConnTimeout"`
	MaxIdleConns    int           `yaml:"maxIdleConns,omitempty" json:"maxIdleConns,omitempty" mapstructure:"maxIdleConns"`
	MaxConnsPerHost int           `yaml:"maxConnsPerHost,omitempty" json:"maxConnsPerHost,omitempty" mapstructure:"maxConnsPerHost"`
	ReadTimeout     time.Duration `yaml:"readTimeout,omitempty" json:"readTimeout,omitempty" mapstructure:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout,omitempty" json:"writeTimeout,omitempty" mapstructure:"writeTimeout"`
	WorkPoolSize    int           `yaml:"workPoolSize,omitempty" json:"workPoolSize,omitempty" mapstructure:"workPoolSize"`
}

type BaseConfig struct {
	ProfPathPrefix string `yaml:"profPathPrefix,omitempty" json:"profPathPrefix,omitempty" mapstructure:"profPathPrefix"`
	LogLevel       string `yaml:"logLevel,omitempty" json:"logLevel,omitempty" mapstructure:"logLevel"`
	ProfEnable     bool   `yaml:"profEnable,omitempty" json:"profEnable,omitempty" mapstructure:"profEnable"`
}

// ServiceQueue 封装底层 ServingServer 并提供 ServiceQueue 抽象，一个 ServiceQueue 可以设置对应的调度策略
type Schedule struct {
	Method            string `yaml:"method,omitempty" json:"method,omitempty" mapstructure:"method"`
	EnableFlowControl bool   `yaml:"enableFlowControl,omitempty" json:"enableFlowControl,omitempty" mapstructure:"enableFlowControl"`
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

func (sg ServiceGroup) ResourceName() string {
	return sg.Name
}

type Queue struct {
	Size int `yaml:"size,omitempty" json:"size,omitempty" mapstructure:"size"` // 服务队列大小
}

type PromMetrics struct {
	Addr           string        `yaml:"addr,omitempty" json:"addr,omitempty" mapstructure:"addr"`
	ScrapeInterval time.Duration `yaml:"scrapeInterval,omitempty" json:"scrapeInterval,omitempty" mapstructure:"scrapeInterval"`
	RemoteWriteURL string        `yaml:"remoteWriteURL,omitempty" json:"remoteWriteURL,omitempty" mapstructure:"remoteWriteURL"`
}
