package config

import (
	"fmt"
	"github.com/edgewize/edgeQ/internal/msc"
	"github.com/ghodss/yaml"
	"io"
	"os"
	"sync"
)

var (
	// singleton instance of config package
	_config = defaultConfig()
)

const (
	// DefaultConfigurationName is the default name of configuration
	defaultConfigurationName = "model-mesh-msc"

	// DefaultConfigurationPath the default location of the configuration file
	defaultConfigurationPath   = "/etc/model-service-controller"
	defaultWebhookCertsDirPath = "/etc/model-service-controller/webhook-certs"

	defaultNamespace = "edgewize-system"

	defaultWebhookServiceName = "modelservice-sidecar-injector"

	defaultWebhookCertFile = defaultWebhookCertsDirPath + "/tls.crt"
	defaultWebhookKeyFile  = defaultWebhookCertsDirPath + "/tls.key"
)

type Config struct {
	Namespace          string             `yaml:"namespace,omitempty"`
	WebhookServiceName string             `yaml:"webhookServiceName,omitempty"`
	WebhookPort        int                `yaml:"webhookPort,omitempty"`
	ProxySidecar       *msc.SidecarConfig `yaml:"proxySidecar,omitempty"`
	BrokerSidecar      *msc.SidecarConfig `yaml:"brokerSidecar,omitempty"`
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

// "viper" cannot parse yaml to struct with `json` tag, so use "github.com/ghodss/yaml" instead
func loadConfig() (*Config, error) {
	configFile, err := os.Open(fmt.Sprintf("%s/%s.yaml", defaultConfigurationPath, defaultConfigurationName))
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
		Namespace:          defaultNamespace,
		WebhookPort:        8444,
		WebhookServiceName: defaultWebhookServiceName,
	}
}

func TryLoadFromDisk() (*Config, error) {
	return _config.loadFromDisk()
}
