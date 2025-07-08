/*
Copyright 2025 The Apache Software Foundation.

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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/config"
	// "github.com/veteran-chad/storm-controller/pkg/coordination" // Not needed for these tests
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// MockStormClientManager is a mock implementation of storm.ClientManager
type MockStormClientManager struct {
	mock.Mock
}

func (m *MockStormClientManager) GetClient() (storm.Client, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(storm.Client), args.Error(1)
}

func (m *MockStormClientManager) UpdateClient(config *storm.ClientConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockStormClientManager) RemoveClient() {
	m.Called()
}

func (m *MockStormClientManager) HasClient() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockStormClusterClient is a mock implementation of storm.Client for clusters
type MockStormClusterClient struct {
	mock.Mock
}

func (m *MockStormClusterClient) GetClusterInfo(ctx context.Context) (*storm.ClusterSummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storm.ClusterSummary), args.Error(1)
}

func (m *MockStormClusterClient) GetTopology(ctx context.Context, name string) (*storm.TopologyInfo, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storm.TopologyInfo), args.Error(1)
}

func (m *MockStormClusterClient) ListTopologies(ctx context.Context) ([]storm.TopologySummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]storm.TopologySummary), args.Error(1)
}

func (m *MockStormClusterClient) ActivateTopology(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockStormClusterClient) DeactivateTopology(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockStormClusterClient) RebalanceTopology(ctx context.Context, name string, options *storm.RebalanceOptions) error {
	args := m.Called(ctx, name, options)
	return args.Error(0)
}

func (m *MockStormClusterClient) GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockStormClusterClient) UploadJar(ctx context.Context, jarPath string, jarData []byte) error {
	args := m.Called(ctx, jarPath, jarData)
	return args.Error(0)
}

func (m *MockStormClusterClient) DownloadJar(ctx context.Context, url string) ([]byte, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStormClusterClient) SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error {
	mockArgs := m.Called(ctx, name, jarPath, mainClass, args, config)
	return mockArgs.Error(0)
}

func (m *MockStormClusterClient) KillTopology(ctx context.Context, name string, timeout int) error {
	args := m.Called(ctx, name, timeout)
	return args.Error(0)
}

func (m *MockStormClusterClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

var _ = Describe("StormClusterReconciler", func() {
	var (
		// reconciler *StormClusterReconciler // Not used in these tests
		k8sClient client.Client
		ctx       context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create scheme
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(stormv1beta1.AddToScheme(scheme)).To(Succeed())

		// Create fake client
		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		// Create mock client manager
		mockClientManager := &MockStormClientManager{}
		mockClient := &MockStormClusterClient{}
		mockClientManager.On("GetClient").Return(mockClient, nil)
		mockClientManager.On("HasClient").Return(true)

		// Mock GetClusterInfo to return a valid cluster summary
		mockClient.On("GetClusterInfo", mock.Anything).Return(&storm.ClusterSummary{
			Supervisors:  3,
			UsedSlots:    10,
			TotalSlots:   20,
			FreeSlots:    10,
			NimbusUptime: 3600,
			Topologies:   2,
			NimbusLeader: "nimbus-1",
			NimbusHosts:  []string{"nimbus-1", "nimbus-2"},
		}, nil)

		// Create coordinator
		// coordinator := coordination.NewResourceCoordinator(k8sClient, mockClientManager, scheme) // Not needed for these tests

		// Create reconciler
		// reconciler = &StormClusterReconciler{
		// 	Client:            k8sClient,
		// 	Scheme:            scheme,
		// 	ClientManager:     mockClientManager,
		// 	Coordinator:       coordinator,
		// 	OperatorNamespace: "storm-operator",
		// } // Not needed for these tests
	})

	Context("When reconciling a StormCluster with config", func() {
		It("Should load operator config and apply defaults", func() {
			// Create operator config ConfigMap
			operatorConfig := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "storm-operator-operator-config",
					Namespace: "storm-operator",
				},
				Data: map[string]string{
					"defaults.yaml": `
defaults:
  storm:
    image:
      registry: custom.registry
      repository: custom/storm
      tag: "2.8.1-custom"
    config:
      nimbus.seeds: ["custom-nimbus"]
      custom.setting: "custom-value"
  cluster:
    nimbus:
      replicas: 3
    supervisor:
      replicas: 5
      slotsPerSupervisor: 6
    ui:
      enabled: true
  zookeeper:
    servers: ["custom-zk:2181"]
`,
				},
			}
			Expect(k8sClient.Create(ctx, operatorConfig)).To(Succeed())

			// Create a StormCluster
			cluster := &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-with-config",
					Namespace: "default",
				},
				Spec: stormv1beta1.StormClusterSpec{
					ManagementMode: "create",
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// Load operator config
			operatorCfg, err := config.LoadOperatorConfig(ctx, k8sClient, "storm-operator")
			Expect(err).NotTo(HaveOccurred())
			Expect(operatorCfg).NotTo(BeNil())

			// Apply defaults to cluster
			config.ApplyDefaults(cluster, operatorCfg)

			// Verify defaults were applied
			Expect(cluster.Spec.Image).NotTo(BeNil())
			Expect(cluster.Spec.Image.Registry).To(Equal("custom.registry"))
			Expect(cluster.Spec.Image.Repository).To(Equal("custom/storm"))
			Expect(cluster.Spec.Image.Tag).To(Equal("2.8.1-custom"))

			Expect(cluster.Spec.Nimbus).NotTo(BeNil())
			Expect(*cluster.Spec.Nimbus.Replicas).To(Equal(int32(3)))

			Expect(cluster.Spec.Supervisor).NotTo(BeNil())
			Expect(*cluster.Spec.Supervisor.Replicas).To(Equal(int32(5)))
			Expect(cluster.Spec.Supervisor.SlotsPerSupervisor).To(Equal(int32(6)))

			Expect(cluster.Spec.UI).NotTo(BeNil())
			Expect(cluster.Spec.UI.Enabled).To(BeTrue())

			Expect(cluster.Spec.Zookeeper).NotTo(BeNil())
			Expect(cluster.Spec.Zookeeper.Servers).To(Equal([]string{"custom-zk:2181"}))
		})

		It("Should handle missing operator config gracefully", func() {
			// Create a StormCluster without operator config
			cluster := &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-no-config",
					Namespace: "default",
				},
				Spec: stormv1beta1.StormClusterSpec{
					ManagementMode: "create",
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// Try to load operator config (should fail)
			_, err := config.LoadOperatorConfig(ctx, k8sClient, "non-existent-namespace")
			Expect(err).To(HaveOccurred())

			// Get default config as fallback
			defaultCfg := config.GetDefaultConfig()
			Expect(defaultCfg).NotTo(BeNil())

			// Apply defaults
			config.ApplyDefaults(cluster, defaultCfg)

			// Verify default values were applied
			Expect(cluster.Spec.Image).NotTo(BeNil())
			Expect(cluster.Spec.Image.Registry).To(Equal("docker.io"))
			Expect(cluster.Spec.Image.Repository).To(Equal("storm"))
			Expect(cluster.Spec.Image.Tag).To(Equal("2.8.1"))

			Expect(cluster.Spec.Nimbus).NotTo(BeNil())
			Expect(*cluster.Spec.Nimbus.Replicas).To(Equal(int32(1)))

			Expect(cluster.Spec.Supervisor).NotTo(BeNil())
			Expect(*cluster.Spec.Supervisor.Replicas).To(Equal(int32(1)))
			Expect(cluster.Spec.Supervisor.SlotsPerSupervisor).To(Equal(int32(1)))

			Expect(cluster.Spec.UI).NotTo(BeNil())
			Expect(cluster.Spec.UI.Enabled).To(BeTrue())

			Expect(cluster.Spec.Zookeeper).NotTo(BeNil())
			Expect(cluster.Spec.Zookeeper.Root).To(Equal("/storm/test-cluster-no-config"))
		})

		It("Should apply config defaults to empty cluster spec", func() {
			// Test config merging functionality
			defaults := &config.OperatorConfig{
				Defaults: config.DefaultsConfig{
					Storm: config.StormDefaults{
						Image: config.ImageDefaults{
							Registry:   "test.registry",
							Repository: "test/storm",
							Tag:        "test-tag",
						},
					},
					Cluster: config.ClusterDefaults{
						Nimbus: config.NimbusDefaults{
							Replicas: 2,
						},
						Supervisor: config.SupervisorDefaults{
							Replicas:           3,
							SlotsPerSupervisor: 4,
						},
						UI: config.UIDefaults{
							Enabled: true,
						},
					},
					Zookeeper: config.ZookeeperDefaults{
						Servers:           []string{"zk1:2181"},
						ConnectionTimeout: 15000,
						SessionTimeout:    20000,
					},
				},
			}

			cluster := &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: stormv1beta1.StormClusterSpec{},
			}

			// Apply defaults
			config.ApplyDefaults(cluster, defaults)

			// Verify defaults were applied
			Expect(cluster.Spec.Image).NotTo(BeNil())
			Expect(cluster.Spec.Image.Registry).To(Equal("test.registry"))
			Expect(cluster.Spec.Image.Repository).To(Equal("test/storm"))
			Expect(cluster.Spec.Image.Tag).To(Equal("test-tag"))

			Expect(cluster.Spec.Nimbus).NotTo(BeNil())
			Expect(cluster.Spec.Nimbus.Replicas).NotTo(BeNil())
			Expect(*cluster.Spec.Nimbus.Replicas).To(Equal(int32(2)))

			Expect(cluster.Spec.Supervisor).NotTo(BeNil())
			Expect(cluster.Spec.Supervisor.Replicas).NotTo(BeNil())
			Expect(*cluster.Spec.Supervisor.Replicas).To(Equal(int32(3)))
			Expect(cluster.Spec.Supervisor.SlotsPerSupervisor).To(Equal(int32(4)))

			Expect(cluster.Spec.UI).NotTo(BeNil())
			Expect(cluster.Spec.UI.Enabled).To(BeTrue())

			Expect(cluster.Spec.Zookeeper).NotTo(BeNil())
			Expect(cluster.Spec.Zookeeper.Servers).To(Equal([]string{"zk1:2181"}))
			Expect(cluster.Spec.Zookeeper.Root).To(Equal("/storm/test-cluster"))
		})

		It("Should merge storm config correctly", func() {
			defaults := map[string]interface{}{
				"nimbus.seeds":    []string{"default-nimbus"},
				"default.setting": "default-value",
				"shared.setting":  "default-shared",
			}

			crdConfig := map[string]interface{}{
				"shared.setting": "overridden-shared",
				"custom.setting": "custom-value",
			}

			merged := config.MergeStormConfig(defaults, crdConfig, "test-cluster")

			// Check that defaults are preserved
			Expect(merged["nimbus.seeds"]).To(Equal([]string{"default-nimbus"}))
			Expect(merged["default.setting"]).To(Equal("default-value"))

			// Check that CRD config overrides defaults
			Expect(merged["shared.setting"]).To(Equal("overridden-shared"))

			// Check that CRD config adds new values
			Expect(merged["custom.setting"]).To(Equal("custom-value"))

			// Check that Zookeeper root is set for cluster isolation
			Expect(merged["storm.zookeeper.root"]).To(Equal("/storm/test-cluster"))
		})
	})
})

func TestStormClusterReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StormClusterReconciler Suite")
}
