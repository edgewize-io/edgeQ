package config

import (
	xconfig "github.com/edgewize/edgeQ/pkg/config"
)

// RPCClient is RPC client config.
type GRPCClient = xconfig.GRPCClient

// RPCServer is RPC server config.
type GRPCServer = xconfig.GRPCServer

type BaseConfig = xconfig.BaseConfig
type Schedule = xconfig.Schedule
type Queue = xconfig.Queue
type Dispatch = xconfig.Dispatch
type ServiceGroup = xconfig.ServiceGroup
type PromMetrics = xconfig.PromMetrics

// Config defines everything needed for apiserver to deal with external services
type Config struct {
	BaseOptions   *BaseConfig     `yaml:"base" json:"baseOptions,omitempty" mapstructure:"base"`
	BrokerServer  *GRPCServer     `yaml:"brokerServer" json:"brokerServer,omitempty" mapstructure:"brokerServer"`
	Schedule      *Schedule       `yaml:"schedule" json:"schedule,omitempty" mapstructure:"schedule"`
	Queue         *Queue          `yaml:"queue" json:"queue,omitempty" mapstructure:"queue"`
	Dispatch      *Dispatch       `yaml:"dispatch" json:"dispatch,omitempty" mapstructure:"dispatch"`
	ServiceGroups []*ServiceGroup `yaml:"serviceGroups" json:"serviceGroups,omitempty" mapstructure:"serviceGroups"`
	PromMetrics   *PromMetrics    `yaml:"promMetrics" json:"promMetrics,omitempty" mapstructure:"promMetrics"`
}
