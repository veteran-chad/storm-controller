package config

import (
	"fmt"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

// MergeStormConfig merges operator defaults with CRD spec config
func MergeStormConfig(defaults map[string]interface{}, crdConfig map[string]interface{}, clusterName string) map[string]interface{} {
	merged := make(map[string]interface{})

	// Start with defaults
	for k, v := range defaults {
		merged[k] = v
	}

	// Override with CRD config
	for k, v := range crdConfig {
		merged[k] = v
	}

	// Ensure cluster-specific Zookeeper root
	if _, exists := merged["storm.zookeeper.root"]; !exists {
		merged["storm.zookeeper.root"] = fmt.Sprintf("/storm/%s", clusterName)
	}

	return merged
}

// ApplyDefaults applies operator defaults to StormCluster if not specified
func ApplyDefaults(cluster *stormv1beta1.StormCluster, defaults *OperatorConfig) {
	// Apply image defaults if not specified
	if cluster.Spec.Image == nil {
		cluster.Spec.Image = &stormv1beta1.ImageSpec{}
	}
	if cluster.Spec.Image.Registry == "" && defaults.Defaults.Storm.Image.Registry != "" {
		cluster.Spec.Image.Registry = defaults.Defaults.Storm.Image.Registry
	}
	if cluster.Spec.Image.Repository == "" && defaults.Defaults.Storm.Image.Repository != "" {
		cluster.Spec.Image.Repository = defaults.Defaults.Storm.Image.Repository
	}
	if cluster.Spec.Image.Tag == "" && defaults.Defaults.Storm.Image.Tag != "" {
		cluster.Spec.Image.Tag = defaults.Defaults.Storm.Image.Tag
	}

	// Apply cluster sizing defaults
	if cluster.Spec.Nimbus == nil {
		cluster.Spec.Nimbus = &stormv1beta1.NimbusSpec{}
	}
	if cluster.Spec.Nimbus.Replicas == nil || *cluster.Spec.Nimbus.Replicas == 0 {
		replicas := defaults.Defaults.Cluster.Nimbus.Replicas
		cluster.Spec.Nimbus.Replicas = &replicas
	}

	if cluster.Spec.Supervisor == nil {
		cluster.Spec.Supervisor = &stormv1beta1.SupervisorSpec{}
	}
	if cluster.Spec.Supervisor.Replicas == nil || *cluster.Spec.Supervisor.Replicas == 0 {
		replicas := defaults.Defaults.Cluster.Supervisor.Replicas
		cluster.Spec.Supervisor.Replicas = &replicas
	}
	if cluster.Spec.Supervisor.SlotsPerSupervisor == 0 {
		cluster.Spec.Supervisor.SlotsPerSupervisor = defaults.Defaults.Cluster.Supervisor.SlotsPerSupervisor
	}

	// Apply UI defaults
	if cluster.Spec.UI == nil {
		cluster.Spec.UI = &stormv1beta1.UISpec{
			Enabled: defaults.Defaults.Cluster.UI.Enabled,
		}
	}

	// Apply Zookeeper defaults if not specified
	if cluster.Spec.Zookeeper == nil {
		cluster.Spec.Zookeeper = &stormv1beta1.ZookeeperSpec{}
	}
	if len(cluster.Spec.Zookeeper.Servers) == 0 && len(defaults.Defaults.Zookeeper.Servers) > 0 {
		cluster.Spec.Zookeeper.Servers = defaults.Defaults.Zookeeper.Servers
	}
	if cluster.Spec.Zookeeper.ConnectionTimeout == 0 {
		cluster.Spec.Zookeeper.ConnectionTimeout = defaults.Defaults.Zookeeper.ConnectionTimeout
	}
	if cluster.Spec.Zookeeper.SessionTimeout == 0 {
		cluster.Spec.Zookeeper.SessionTimeout = defaults.Defaults.Zookeeper.SessionTimeout
	}

	// Ensure Zookeeper root path isolation
	if cluster.Spec.Zookeeper.Root == "" {
		cluster.Spec.Zookeeper.Root = fmt.Sprintf("/storm/%s", cluster.Name)
	}
}

// MergeImageDefaults merges image configuration with defaults
func MergeImageDefaults(imageSpec *stormv1beta1.ImageSpec, defaults ImageDefaults) {
	if imageSpec == nil {
		return
	}

	if imageSpec.Registry == "" {
		imageSpec.Registry = defaults.Registry
	}
	if imageSpec.Repository == "" {
		imageSpec.Repository = defaults.Repository
	}
	if imageSpec.Tag == "" {
		imageSpec.Tag = defaults.Tag
	}
}

// GetZookeeperConfig returns the Zookeeper configuration for Storm
func GetZookeeperConfig(cluster *stormv1beta1.StormCluster) map[string]interface{} {
	config := make(map[string]interface{})

	// Set Zookeeper servers
	config["storm.zookeeper.servers"] = cluster.Spec.Zookeeper.Servers

	// Set Zookeeper root path with cluster isolation
	config["storm.zookeeper.root"] = cluster.Spec.Zookeeper.Root

	// Set timeouts
	if cluster.Spec.Zookeeper.ConnectionTimeout > 0 {
		config["storm.zookeeper.connection.timeout"] = cluster.Spec.Zookeeper.ConnectionTimeout
	}
	if cluster.Spec.Zookeeper.SessionTimeout > 0 {
		config["storm.zookeeper.session.timeout"] = cluster.Spec.Zookeeper.SessionTimeout
	}

	return config
}
