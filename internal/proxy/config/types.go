package config

import (
	xconfig "github.com/edgewize/edgeQ/pkg/config"
)

type MeshProxy = xconfig.MeshProxy
type BaseConfig = xconfig.BaseConfig
type ServiceGroup = xconfig.ServiceGroup
type GRPCConfig = xconfig.GRPCConfig
type HTTPConfig = xconfig.HTTPConfig

// Config defines everything needed for apiserver to deal with external services
type Config struct {
	BaseOptions  BaseConfig   `yaml:"base,omitempty" json:"base,omitempty" mapstructure:"base"`
	Proxy        MeshProxy    `yaml:"proxyServer,omitempty" json:"proxyServer,omitempty" mapstructure:"proxyServer"`
	ServiceGroup ServiceGroup `yaml:"serviceGroup,omitempty" json:"serviceGroup,omitempty" mapstructure:"serviceGroup"`
}
