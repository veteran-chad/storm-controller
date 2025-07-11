package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadOperatorConfig(t *testing.T) {
	tests := []struct {
		name          string
		configData    string
		expectError   bool
		expectedImage ImageDefaults
	}{
		{
			name: "valid config",
			configData: `defaults:
  storm:
    image:
      registry: "custom.registry"
      repository: "custom/storm"
      tag: "3.0.0"
    config:
      nimbus.seeds: ["custom-nimbus"]
  cluster:
    nimbus:
      replicas: 2
    supervisor:
      replicas: 5
      slotsPerSupervisor: 8
  zookeeper:
    servers: ["zk1:2181", "zk2:2181"]
    connectionTimeout: 30000
    sessionTimeout: 40000`,
			expectError: false,
			expectedImage: ImageDefaults{
				Registry:   "custom.registry",
				Repository: "custom/storm",
				Tag:        "3.0.0",
			},
		},
		{
			name:        "invalid yaml",
			configData:  "invalid yaml content [",
			expectError: true,
		},
		{
			name:        "empty config",
			configData:  "",
			expectError: false, // Empty YAML is valid, creates zero-value struct
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create ConfigMap
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "storm-operator-operator-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"defaults.yaml": tt.configData,
				},
			}

			// Create fake client
			scheme := runtime.NewScheme()
			assert.NoError(t, corev1.AddToScheme(scheme))
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cm).
				Build()

			// Test
			config, err := LoadOperatorConfig(context.TODO(), client, "test-namespace")

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.expectedImage, config.Defaults.Storm.Image)
			}
		})
	}
}

func TestLoadOperatorConfigMissingConfigMap(t *testing.T) {
	// Create fake client without ConfigMap
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	config, err := LoadOperatorConfig(context.TODO(), client, "test-namespace")

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "failed to get operator config")
}

func TestLoadOperatorConfigMissingDefaultsYaml(t *testing.T) {
	// Create ConfigMap without defaults.yaml
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storm-operator-operator-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"other.yaml": "some data",
		},
	}

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cm).
		Build()

	config, err := LoadOperatorConfig(context.TODO(), client, "test-namespace")

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "defaults.yaml not found in configmap")
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "docker.io", config.Defaults.Storm.Image.Registry)
	assert.Equal(t, "storm", config.Defaults.Storm.Image.Repository)
	assert.Equal(t, "2.8.1", config.Defaults.Storm.Image.Tag)
	assert.Equal(t, int32(1), config.Defaults.Cluster.Nimbus.Replicas)
	assert.Equal(t, int32(1), config.Defaults.Cluster.Supervisor.Replicas)
	assert.Equal(t, int32(1), config.Defaults.Cluster.Supervisor.SlotsPerSupervisor)
	assert.True(t, config.Defaults.Cluster.UI.Enabled)
	assert.Equal(t, []string{"zookeeper:2181"}, config.Defaults.Zookeeper.Servers)
	assert.Equal(t, 15000, config.Defaults.Zookeeper.ConnectionTimeout)
	assert.Equal(t, 20000, config.Defaults.Zookeeper.SessionTimeout)
}
