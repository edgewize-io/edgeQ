package config

import (
	"fmt"
	"github.com/edgewize/edgeQ/pkg/simple/client/k8s"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"reflect"
	"strings"
	"sync"
)

var (
	defaultUpdater = getDefaultConfigUpdater()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "edgeQapiServer"

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath = "/etc/edgeQapiServer"
)

type APIServerConfig struct {
	KubernetesOptions *k8s.KubernetesOptions `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty" mapstructure:"kubernetes"`
}

type configUpdater struct {
	cfg         *APIServerConfig
	cfgChangeCh chan APIServerConfig
	watchOnce   sync.Once
	loadOnce    sync.Once
}

func (cu *configUpdater) watchConfig() <-chan APIServerConfig {
	cu.watchOnce.Do(func() {
		viper.WatchConfig()
		viper.OnConfigChange(func(in fsnotify.Event) {
			cfg := New()
			if err := viper.Unmarshal(cfg); err != nil {
				klog.Warning("config reload error", err)
			} else {
				cu.cfgChangeCh <- *cfg
			}
		})
	})
	return cu.cfgChangeCh
}

func (cu *configUpdater) loadFromDisk() (*APIServerConfig, error) {
	var err error
	cu.loadOnce.Do(func() {
		if err = viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				err = fmt.Errorf("error parsing configuration file %s", err)
			}
		}
		err = viper.Unmarshal(cu.cfg)
	})
	return cu.cfg, err
}

func getDefaultConfigUpdater() *configUpdater {
	viper.SetConfigName(defaultConfigurationName)
	viper.AddConfigPath(defaultConfigurationPath)

	// Load from current working directory, only used for debugging
	viper.AddConfigPath(".")

	// Load from Environment variables
	viper.SetEnvPrefix("kubesphere")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return &configUpdater{
		cfg:         New(),
		cfgChangeCh: make(chan APIServerConfig),
		watchOnce:   sync.Once{},
		loadOnce:    sync.Once{},
	}
}

// TryLoadFromDisk loads configuration from default location after server startup
// return nil error if configuration file not exists
func TryLoadFromDisk() (*APIServerConfig, error) {
	return defaultUpdater.loadFromDisk()
}

// WatchConfigChange return config change channel
func WatchConfigChange() <-chan APIServerConfig {
	return defaultUpdater.watchConfig()
}

// convertToMap simply converts config to map[string]bool
// to hide sensitive information
func (conf *APIServerConfig) ToMap() map[string]bool {
	result := make(map[string]bool, 0)

	if conf == nil {
		return result
	}

	c := reflect.Indirect(reflect.ValueOf(conf))

	for i := 0; i < c.NumField(); i++ {
		name := strings.Split(c.Type().Field(i).Tag.Get("json"), ",")[0]
		if strings.HasPrefix(name, "-") {
			continue
		}

		if c.Field(i).IsNil() {
			result[name] = false
		} else {
			result[name] = true
		}
	}

	return result
}

func New() *APIServerConfig {
	return &APIServerConfig{
		KubernetesOptions: k8s.NewKubernetesOptions(),
	}
}
