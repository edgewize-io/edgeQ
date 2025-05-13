package config

import (
	xconfig "github.com/edgewize/edgeQ/pkg/config"
)

type MeshServer = xconfig.MeshServer
type BaseConfig = xconfig.BaseConfig
type Schedule = xconfig.Schedule
type Queue = xconfig.Queue
type ServiceGroup = xconfig.ServiceGroup
type PromMetrics = xconfig.PromMetrics
type HTTPConfig = xconfig.HTTPConfig
type GRPCConfig = xconfig.GRPCConfig

type Config struct {
	BaseOptions   BaseConfig     `yaml:"base" json:"baseOptions,omitempty" mapstructure:"base"`
	Broker        MeshServer     `yaml:"broker" json:"broker,omitempty" mapstructure:"broker"`
	Schedule      Schedule       `yaml:"schedule" json:"schedule,omitempty" mapstructure:"schedule"`
	Queue         Queue          `yaml:"queue" json:"queue,omitempty" mapstructure:"queue"`
	ServiceGroups []ServiceGroup `yaml:"serviceGroups" json:"serviceGroups,omitempty" mapstructure:"serviceGroups"`
	PromMetrics   PromMetrics    `yaml:"promMetrics" json:"promMetrics,omitempty" mapstructure:"promMetrics"`
}
