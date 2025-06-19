package config

import (
	"fmt"
	"os"

	"github.com/llmariner/cluster-manager/pkg/status"
	"gopkg.in/yaml.v3"
)

// KubernetesManagerConfig is the Kubernetes manager configuration.
type KubernetesManagerConfig struct {
	EnableLeaderElection bool   `yaml:"enableLeaderElection"`
	LeaderElectionID     string `yaml:"leaderElectionID"`

	MetricsBindAddress string `yaml:"metricsBindAddress"`
	HealthBindAddress  string `yaml:"healthBindAddress"`
	PprofBindAddress   string `yaml:"pprofBindAddress"`
}

func (c *KubernetesManagerConfig) validate() error {
	if c.EnableLeaderElection && c.LeaderElectionID == "" {
		return fmt.Errorf("leader election ID must be set")
	}
	return nil
}

// WorkerTLSConfig is the worker TLS configuration.
type WorkerTLSConfig struct {
	Enable bool `yaml:"enable"`
}

// WorkerConfig is the worker configuration.
type WorkerConfig struct {
	TLS WorkerTLSConfig `yaml:"tls"`
}

// Config is the configuration.
type Config struct {
	ClusterMonitorServerWorkerServiceAddr string `yaml:"clusterMonitorServerWorkerServiceAddr"`

	KubernetesManager KubernetesManagerConfig `yaml:"kubernetesManager"`

	Worker WorkerConfig `yaml:"worker"`

	// ComponentStatusSender is the configuration for the component status sender.
	ComponentStatusSender status.Config `yaml:"componentStatusSender"`
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ClusterMonitorServerWorkerServiceAddr == "" {
		return fmt.Errorf("cluster monitor server worker service address must be set")
	}

	if err := c.KubernetesManager.validate(); err != nil {
		return fmt.Errorf("kubernetes manager: %s", err)
	}

	if err := c.ComponentStatusSender.Validate(); err != nil {
		return fmt.Errorf("componentStatusSender: %s", err)
	}

	return nil
}

// Parse parses the configuration file at the given path, returning a new
// Config struct.
func Parse(path string) (Config, error) {
	var config Config

	b, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("config: read: %s", err)
	}

	if err = yaml.Unmarshal(b, &config); err != nil {
		return config, fmt.Errorf("config: unmarshal: %s", err)
	}
	return config, nil
}
