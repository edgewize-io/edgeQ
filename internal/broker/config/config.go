/*
Copyright 2020 The KubeSphere Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"github.com/edgewize/edgeQ/pkg/constants"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

// Package config saves configuration for running ModelMesh components
//
// Config can be configured from command line flags and configuration file.
// Command line flags hold higher priority than configuration file. But if
// component Endpoint/Host/APIServer was left empty, all of that component
// command line flags will be ignored, use configuration file instead.
// For example, we have configuration file
//
// mysql:
//   host: mysql.kubesphere-system.svc
//   username: root
//   password: password
//
// At the same time, have command line flags like following:
//
// --mysql-host mysql.openpitrix-system.svc --mysql-username king --mysql-password 1234
//
// We will use `king:1234@mysql.openpitrix-system.svc` from command line flags rather
// than `root:password@mysql.kubesphere-system.svc` from configuration file,
// cause command line has higher priority. But if command line flags like following:
//
// --mysql-username root --mysql-password password
//
// we will `root:password@mysql.kubesphere-system.svc` as input, cause
// mysql-host is missing in command line flags, all other mysql command line flags
// will be ignored.

var (
	// singleton instance of config package
	_config = defaultConfig()
	_viper  = viper.New()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "edge-qos-broker"

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath = "/etc/edge-qos-broker"
)

var defaultBrokerAddr = fmt.Sprintf("0.0.0.0:%s", constants.DefaultBrokerContainerPort)

type config struct {
	cfg         *Config
	cfgChangeCh chan Config
	watchOnce   sync.Once
	loadOnce    sync.Once
}

func (c *config) watchConfig() <-chan Config {
	c.watchOnce.Do(func() {
		_viper.WatchConfig()
		_viper.OnConfigChange(func(in fsnotify.Event) {
			cfg := New()
			if err := _viper.Unmarshal(cfg); err != nil {
				klog.Warningf("config reload error: %v", err)
			} else {
				c.cfgChangeCh <- *cfg
			}
		})
	})
	return c.cfgChangeCh
}

func (c *config) loadFromDisk() (*Config, error) {
	var err error
	c.loadOnce.Do(func() {
		if err = _viper.ReadInConfig(); err != nil {
			return
		}
		err = _viper.Unmarshal(c.cfg)
	})
	return c.cfg, err
}

func defaultConfig() *config {
	_viper.SetConfigName(defaultConfigurationName)
	_viper.AddConfigPath(defaultConfigurationPath)

	// Load from current working directory, only used for debugging
	_viper.AddConfigPath(".")
	_viper.SetConfigType("yaml")

	// Load from Environment variables
	_viper.SetEnvPrefix("kubesphere")
	_viper.AutomaticEnv()
	_viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return &config{
		cfg:         New(),
		cfgChangeCh: make(chan Config),
		watchOnce:   sync.Once{},
		loadOnce:    sync.Once{},
	}
}

// New config creates a default non-empty Config
func New() *Config {
	return &Config{
		BaseOptions: &BaseConfig{
			LogLevel:       "info",
			WhiteList:      []string{},
			ProfEnable:     false,
			TracerEnable:   false,
			ProfPathPrefix: "debug",
			BaseConfig:     "",
		},
		BrokerServer: &GRPCServer{
			Addr:                 defaultBrokerAddr,
			Timeout:              time.Second * 1,
			IdleTimeout:          time.Second * 60,
			MaxLifeTime:          time.Hour * 2,
			ForceCloseWait:       time.Second * 20,
			KeepAliveInterval:    time.Second * 60,
			KeepAliveTimeout:     time.Second * 20,
			MaxMessageSize:       102428800,
			MaxConcurrentStreams: 1024,
		},
		Schedule: &Schedule{
			Method: "WRR",
		},
		Queue: &Queue{
			Size: 10,
		},
		Dispatch: &Dispatch{
			PoolSize: 10,
			Client: &GRPCClient{
				Addr:    defaultBrokerAddr,
				Timeout: time.Second * 5,
			},
			Queue: &Queue{
				Size: 10,
			},
		},
		ServiceGroups: []*ServiceGroup{
			{
				Name:        "default",
				Reclaimable: true,
				Weight:      100,
			},
			{
				Name:        "default1",
				Reclaimable: true,
				Weight:      100,
			},
		},
		PromMetrics: &PromMetrics{
			Addr:           fmt.Sprintf("0.0.0.0:%d", constants.DefaultMetricPort),
			ScrapeInterval: constants.DefaultInterval,
		},
	}
}

// TryLoadFromDisk loads configuration from default location after server startup
// return nil error if configuration file not exists
func TryLoadFromDisk() (*Config, error) {
	return _config.loadFromDisk()
}

// WatchConfigChange return config change channel
func WatchConfigChange() <-chan Config {
	return _config.watchConfig()
}
