package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

func TestMergeStormConfig(t *testing.T) {
	tests := []struct {
		name         string
		defaults     map[string]interface{}
		crdConfig    map[string]interface{}
		clusterName  string
		expectedRoot string
		expectedKey  string
		expectedVal  interface{}
	}{
		{
			name: "default zookeeper root",
			defaults: map[string]interface{}{
				"nimbus.seeds": []string{"nimbus"},
				"custom.key":   "default-value",
			},
			crdConfig:    map[string]interface{}{},
			clusterName:  "test-cluster",
			expectedRoot: "/storm/test-cluster",
			expectedKey:  "custom.key",
			expectedVal:  "default-value",
		},
		{
			name: "custom zookeeper root",
			defaults: map[string]interface{}{
				"nimbus.seeds": []string{"nimbus"},
			},
			crdConfig: map[string]interface{}{
				"storm.zookeeper.root": "/custom/path",
			},
			clusterName:  "test-cluster",
			expectedRoot: "/custom/path",
		},
		{
			name: "crd overrides defaults",
			defaults: map[string]interface{}{
				"custom.key": "default-value",
				"other.key":  "other-default",
			},
			crdConfig: map[string]interface{}{
				"custom.key": "overridden-value",
			},
			clusterName:  "test-cluster",
			expectedRoot: "/storm/test-cluster",
			expectedKey:  "custom.key",
			expectedVal:  "overridden-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeStormConfig(tt.defaults, tt.crdConfig, tt.clusterName)

			// Check Zookeeper root
			root, exists := merged["storm.zookeeper.root"]
			assert.True(t, exists)
			assert.Equal(t, tt.expectedRoot, root)

			// Check custom key if specified
			if tt.expectedKey != "" {
				val, exists := merged[tt.expectedKey]
				assert.True(t, exists)
				assert.Equal(t, tt.expectedVal, val)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	defaults := &OperatorConfig{
		Defaults: DefaultsConfig{
			Storm: StormDefaults{
				Image: ImageDefaults{
					Registry:   "custom.registry",
					Repository: "custom/storm",
					Tag:        "3.0.0",
				},
			},
			Cluster: ClusterDefaults{
				Nimbus: NimbusDefaults{
					Replicas: 2,
				},
				Supervisor: SupervisorDefaults{
					Replicas:           5,
					SlotsPerSupervisor: 8,
				},
				UI: UIDefaults{
					Enabled: true,
				},
			},
			Zookeeper: ZookeeperDefaults{
				Servers:           []string{"zk1:2181", "zk2:2181"},
				ConnectionTimeout: 30000,
				SessionTimeout:    40000,
			},
		},
	}

	tests := []struct {
		name            string
		cluster         *stormv1beta1.StormCluster
		expectedNimbus  int32
		expectedSupReps int32
		expectedSlots   int32
		expectedImage   string
	}{
		{
			name: "empty cluster gets all defaults",
			cluster: &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: stormv1beta1.StormClusterSpec{},
			},
			expectedNimbus:  2,
			expectedSupReps: 5,
			expectedSlots:   8,
			expectedImage:   "custom.registry/custom/storm:3.0.0",
		},
		{
			name: "partial cluster spec keeps existing values",
			cluster: &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: stormv1beta1.StormClusterSpec{
					Nimbus: &stormv1beta1.NimbusSpec{
						Replicas: int32Ptr(3),
					},
					Image: &stormv1beta1.ImageSpec{
						Tag: "4.0.0",
					},
				},
			},
			expectedNimbus:  3,                                    // Keeps existing value
			expectedSupReps: 5,                                    // Gets default
			expectedSlots:   8,                                    // Gets default
			expectedImage:   "custom.registry/custom/storm:4.0.0", // Partial override
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyDefaults(tt.cluster, defaults)

			// Check Nimbus replicas
			assert.NotNil(t, tt.cluster.Spec.Nimbus)
			assert.NotNil(t, tt.cluster.Spec.Nimbus.Replicas)
			assert.Equal(t, tt.expectedNimbus, *tt.cluster.Spec.Nimbus.Replicas)

			// Check Supervisor replicas and slots
			assert.NotNil(t, tt.cluster.Spec.Supervisor)
			assert.NotNil(t, tt.cluster.Spec.Supervisor.Replicas)
			assert.Equal(t, tt.expectedSupReps, *tt.cluster.Spec.Supervisor.Replicas)
			assert.Equal(t, tt.expectedSlots, tt.cluster.Spec.Supervisor.SlotsPerSupervisor)

			// Check Image
			assert.NotNil(t, tt.cluster.Spec.Image)
			imageStr := tt.cluster.Spec.Image.Registry + "/" + tt.cluster.Spec.Image.Repository + ":" + tt.cluster.Spec.Image.Tag
			assert.Equal(t, tt.expectedImage, imageStr)

			// Check UI enabled
			assert.NotNil(t, tt.cluster.Spec.UI)
			assert.True(t, tt.cluster.Spec.UI.Enabled)

			// Check Zookeeper
			assert.NotNil(t, tt.cluster.Spec.Zookeeper)
			assert.Equal(t, defaults.Defaults.Zookeeper.Servers, tt.cluster.Spec.Zookeeper.Servers)
			assert.Equal(t, "/storm/test-cluster", tt.cluster.Spec.Zookeeper.Root)
		})
	}
}

func TestMergeImageDefaults(t *testing.T) {
	defaults := ImageDefaults{
		Registry:   "default.registry",
		Repository: "default/repo",
		Tag:        "1.0.0",
	}

	tests := []struct {
		name          string
		imageSpec     *stormv1beta1.ImageSpec
		expectedImage *stormv1beta1.ImageSpec
	}{
		{
			name:          "nil image spec",
			imageSpec:     nil,
			expectedImage: nil,
		},
		{
			name:      "empty image spec gets all defaults",
			imageSpec: &stormv1beta1.ImageSpec{},
			expectedImage: &stormv1beta1.ImageSpec{
				Registry:   "default.registry",
				Repository: "default/repo",
				Tag:        "1.0.0",
			},
		},
		{
			name: "partial image spec gets remaining defaults",
			imageSpec: &stormv1beta1.ImageSpec{
				Tag: "2.0.0",
			},
			expectedImage: &stormv1beta1.ImageSpec{
				Registry:   "default.registry",
				Repository: "default/repo",
				Tag:        "2.0.0",
			},
		},
		{
			name: "full image spec unchanged",
			imageSpec: &stormv1beta1.ImageSpec{
				Registry:   "custom.registry",
				Repository: "custom/repo",
				Tag:        "3.0.0",
			},
			expectedImage: &stormv1beta1.ImageSpec{
				Registry:   "custom.registry",
				Repository: "custom/repo",
				Tag:        "3.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeImageDefaults(tt.imageSpec, defaults)

			if tt.expectedImage == nil {
				assert.Nil(t, tt.imageSpec)
			} else {
				assert.Equal(t, tt.expectedImage, tt.imageSpec)
			}
		})
	}
}

func TestGetZookeeperConfig(t *testing.T) {
	cluster := &stormv1beta1.StormCluster{
		Spec: stormv1beta1.StormClusterSpec{
			Zookeeper: &stormv1beta1.ZookeeperSpec{
				Servers:           []string{"zk1:2181", "zk2:2181"},
				Root:              "/storm/test-cluster",
				ConnectionTimeout: 15000,
				SessionTimeout:    20000,
			},
		},
	}

	config := GetZookeeperConfig(cluster)

	expected := map[string]interface{}{
		"storm.zookeeper.servers":            []string{"zk1:2181", "zk2:2181"},
		"storm.zookeeper.port":               2181,
		"storm.zookeeper.root":               "/storm/test-cluster",
		"storm.zookeeper.connection.timeout": 15000,
		"storm.zookeeper.session.timeout":    20000,
	}

	assert.Equal(t, expected, config)
}

func TestGetZookeeperConfigWithoutTimeouts(t *testing.T) {
	cluster := &stormv1beta1.StormCluster{
		Spec: stormv1beta1.StormClusterSpec{
			Zookeeper: &stormv1beta1.ZookeeperSpec{
				Servers: []string{"zk:2181"},
				Root:    "/storm/test",
				// No timeouts set (should be 0)
			},
		},
	}

	config := GetZookeeperConfig(cluster)

	expected := map[string]interface{}{
		"storm.zookeeper.servers": []string{"zk:2181"},
		"storm.zookeeper.port":    2181,
		"storm.zookeeper.root":    "/storm/test",
	}

	assert.Equal(t, expected, config)

	// Ensure timeout keys are not present when values are 0
	_, hasConnTimeout := config["storm.zookeeper.connection.timeout"]
	_, hasSessionTimeout := config["storm.zookeeper.session.timeout"]
	assert.False(t, hasConnTimeout)
	assert.False(t, hasSessionTimeout)
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}
