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
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"strings"
	"sync"
	"time"
)

var (
	// singleton instance of config package
	_config = defaultConfig()
	_viper  = viper.New()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "edge-qos-proxy"

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath = "/etc/edge-qos-proxy"
)

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
		BaseOptions: BaseConfig{
			LogLevel:       "info",
			ProfEnable:     false,
			ProfPathPrefix: "debug",
		},
		Proxy: MeshProxy{
			Addr:            fmt.Sprintf("%s:%s", "127.0.0.1", constants.DefaultProxyContainerPort),
			DialTimeout:     time.Second * 30,
			KeepAlive:       time.Second * 30,
			HeaderTimeout:   time.Second * 20,
			IdleConnTimeout: time.Second * 120,
			MaxIdleConns:    1000,
			MaxConnsPerHost: 1000,
			ReadTimeout:     time.Second * 30,
			WriteTimeout:    time.Second * 30,
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
