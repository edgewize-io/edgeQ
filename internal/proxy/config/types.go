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

// Config defines everything needed for apiserver to deal with external services
type Config struct {
	BaseOptions  *BaseConfig   `yaml:"base,omitempty" json:"base,omitempty" mapstructure:"base"`
	ProxyServer  *GRPCServer   `yaml:"proxyServer,omitempty" json:"proxyServer,omitempty" mapstructure:"proxyServer"`
	ServiceGroup *ServiceGroup `yaml:"serviceGroup,omitempty" json:"serviceGroup,omitempty" mapstructure:"serviceGroup"`
	Dispatch     *Dispatch     `yaml:"dispatch,omitempty" json:"dispatch,omitempty" mapstructure:"dispatch"`
}
