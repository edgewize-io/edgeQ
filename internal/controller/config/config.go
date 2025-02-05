package config

import (
	"fmt"
	"github.com/edgewize/edgeQ/internal/controller"
	"github.com/edgewize/edgeQ/pkg/simple/client/k8s"
	"github.com/ghodss/yaml"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	// singleton instance of config package
	_config = defaultConfig()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "edge-qos-controller"

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath = "/etc/edge-qos-controller"

	defaultWebhookServiceName = "edge-qos-webhook"
)

type Config struct {
	KubernetesOptions  *k8s.KubernetesOptions    `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty" mapstructure:"kubernetes"`
	WebhookServiceName string                    `yaml:"webhookServiceName,omitempty"`
	WebhookPort        int                       `yaml:"webhookPort,omitempty"`
	ProxySidecar       *controller.SidecarConfig `yaml:"proxySidecar,omitempty"`
	BrokerSidecar      *controller.SidecarConfig `yaml:"brokerSidecar,omitempty"`
}

type config struct {
	cfg         *Config
	cfgChangeCh chan Config
	watchOnce   sync.Once
	loadOnce    sync.Once
}

func (c *config) loadFromDisk() (*Config, error) {
	var err error
	c.loadOnce.Do(func() {
		c.cfg, err = loadConfig()
	})
	return c.cfg, err
}

func getConfigPath() string {
	fileName := fmt.Sprintf("%s.yaml", defaultConfigurationName)

	debug := os.Getenv("DEBUG")
	if debug != "" {
		return filepath.Join("tmp", fileName)
	}

	return fmt.Sprintf("%s/%s.yaml", defaultConfigurationPath, defaultConfigurationName)
}

// "viper" cannot parse yaml to struct with `json` tag, so use "github.com/ghodss/yaml" instead
func loadConfig() (*Config, error) {
	configFilePath := getConfigPath()
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()
	data, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func defaultConfig() *config {
	return &config{
		cfg:         New(),
		cfgChangeCh: make(chan Config),
		watchOnce:   sync.Once{},
		loadOnce:    sync.Once{},
	}
}

func New() *Config {
	return &Config{
		KubernetesOptions:  k8s.NewKubernetesOptions(),
		WebhookPort:        8444,
		WebhookServiceName: defaultWebhookServiceName,
	}
}

func TryLoadFromDisk() (*Config, error) {
	return _config.loadFromDisk()
}
