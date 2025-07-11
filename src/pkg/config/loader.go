package config

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// OperatorConfig represents the operator configuration loaded from ConfigMap
type OperatorConfig struct {
	Defaults DefaultsConfig `yaml:"defaults"`
}

// DefaultsConfig contains default settings for Storm clusters
type DefaultsConfig struct {
	Storm     StormDefaults     `yaml:"storm"`
	Cluster   ClusterDefaults   `yaml:"cluster"`
	Zookeeper ZookeeperDefaults `yaml:"zookeeper"`
}

// StormDefaults contains default Storm settings
type StormDefaults struct {
	Image  ImageDefaults          `yaml:"image"`
	Config map[string]interface{} `yaml:"config"`
}

// ImageDefaults contains default image settings
type ImageDefaults struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
}

// ClusterDefaults contains default cluster sizing
type ClusterDefaults struct {
	Nimbus     NimbusDefaults     `yaml:"nimbus"`
	Supervisor SupervisorDefaults `yaml:"supervisor"`
	UI         UIDefaults         `yaml:"ui"`
}

// NimbusDefaults contains default Nimbus settings
type NimbusDefaults struct {
	Replicas int32 `yaml:"replicas"`
}

// SupervisorDefaults contains default Supervisor settings
type SupervisorDefaults struct {
	Replicas           int32 `yaml:"replicas"`
	SlotsPerSupervisor int32 `yaml:"slotsPerSupervisor"`
}

// UIDefaults contains default UI settings
type UIDefaults struct {
	Enabled bool `yaml:"enabled"`
}

// ZookeeperDefaults contains default Zookeeper settings
type ZookeeperDefaults struct {
	Servers           []string `yaml:"servers"`
	ConnectionTimeout int      `yaml:"connectionTimeout"`
	SessionTimeout    int      `yaml:"sessionTimeout"`
}

// LoadOperatorConfig loads the operator configuration from a ConfigMap
func LoadOperatorConfig(ctx context.Context, c client.Client, namespace string) (*OperatorConfig, error) {
	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      "storm-operator-operator-config",
		Namespace: namespace,
	}, cm)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator config: %w", err)
	}

	configData, ok := cm.Data["defaults.yaml"]
	if !ok {
		return nil, fmt.Errorf("defaults.yaml not found in configmap")
	}

	config := &OperatorConfig{}
	if err := yaml.Unmarshal([]byte(configData), config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// GetDefaultConfig returns a default configuration if no ConfigMap is found
func GetDefaultConfig() *OperatorConfig {
	return &OperatorConfig{
		Defaults: DefaultsConfig{
			Storm: StormDefaults{
				Image: ImageDefaults{
					Registry:   "docker.io",
					Repository: "storm",
					Tag:        "2.8.1",
				},
				Config: map[string]interface{}{
					"nimbus.seeds":       []string{"nimbus"},
					"storm.local.dir":    "/storm/data",
					"storm.log.dir":      "/logs",
					"nimbus.thrift.port": 6627,
				},
			},
			Cluster: ClusterDefaults{
				Nimbus: NimbusDefaults{
					Replicas: 1,
				},
				Supervisor: SupervisorDefaults{
					Replicas:           1,
					SlotsPerSupervisor: 1,
				},
				UI: UIDefaults{
					Enabled: true,
				},
			},
			Zookeeper: ZookeeperDefaults{
				Servers:           []string{"zookeeper:2181"},
				ConnectionTimeout: 15000,
				SessionTimeout:    20000,
			},
		},
	}
}
