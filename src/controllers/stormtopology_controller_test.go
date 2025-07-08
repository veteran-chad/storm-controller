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
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/coordination"
	"github.com/veteran-chad/storm-controller/pkg/jarextractor"
	"github.com/veteran-chad/storm-controller/pkg/state"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// MockStormClient is a mock implementation of storm.Client
type MockStormClient struct {
	mock.Mock
}

func (m *MockStormClient) GetTopology(ctx context.Context, name string) (*storm.TopologyInfo, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storm.TopologyInfo), args.Error(1)
}

func (m *MockStormClient) KillTopology(ctx context.Context, name string, waitSecs int) error {
	args := m.Called(ctx, name, waitSecs)
	return args.Error(0)
}

func (m *MockStormClient) DownloadJar(ctx context.Context, url string) ([]byte, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockStormClient) DeactivateTopology(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockStormClient) ActivateTopology(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockStormClient) GetClusterInfo(ctx context.Context) (*storm.ClusterSummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storm.ClusterSummary), args.Error(1)
}

func (m *MockStormClient) GetClusterConfiguration(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockStormClient) SubmitTopology(ctx context.Context, name string, jarPath string, mainClass string, args []string, config map[string]string) error {
	mockArgs := m.Called(ctx, name, jarPath, mainClass, args, config)
	return mockArgs.Error(0)
}

func (m *MockStormClient) RebalanceTopology(ctx context.Context, name string, options *storm.RebalanceOptions) error {
	args := m.Called(ctx, name, options)
	return args.Error(0)
}

func (m *MockStormClient) ListTopologies(ctx context.Context) ([]storm.TopologySummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]storm.TopologySummary), args.Error(1)
}

func (m *MockStormClient) UploadJar(ctx context.Context, jarPath string, jarData []byte) error {
	args := m.Called(ctx, jarPath, jarData)
	return args.Error(0)
}

// MockClientManager is a mock implementation of storm.ClientManager
type MockClientManager struct {
	mock.Mock
}

func (m *MockClientManager) GetClient() (storm.Client, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(storm.Client), args.Error(1)
}

func (m *MockClientManager) UpdateClient(config *storm.ClientConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockClientManager) RemoveClient() {
	m.Called()
}

func (m *MockClientManager) HasClient() bool {
	args := m.Called()
	return args.Bool(0)
}

var _ = Describe("StormTopology State Machine Controller", func() {
	var (
		reconciler      *StormTopologyReconciler
		fakeClient      client.Client
		mockStormClient *MockStormClient
		mockClientMgr   *MockClientManager
		ctx             context.Context
		scheme          *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(stormv1beta1.AddToScheme(scheme)).To(Succeed())

		// Create fake client
		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&stormv1beta1.StormTopology{}).
			WithStatusSubresource(&stormv1beta1.StormCluster{}).
			Build()

		// Create mocks
		mockStormClient = new(MockStormClient)
		mockClientMgr = new(MockClientManager)
		mockClientMgr.On("GetClient").Return(mockStormClient, nil)

		// Create reconciler
		reconciler = &StormTopologyReconciler{
			Client:        fakeClient,
			Scheme:        scheme,
			ClientManager: mockClientMgr,
			JarExtractor:  jarextractor.NewExtractor(fakeClient, "storm-system"),
			ClusterName:   "test-cluster",
			Namespace:     "storm-system",
			Coordinator:   coordination.NewResourceCoordinator(fakeClient, mockClientMgr, scheme),
		}
	})

	Describe("State Machine Initialization", func() {
		It("should initialize state machine from Unknown state", func() {
			topology := &stormv1beta1.StormTopology{
				Status: stormv1beta1.StormTopologyStatus{
					Phase: "",
				},
			}
			sm := reconciler.initializeStateMachine(topology)
			Expect(string(sm.CurrentState())).To(Equal("Unknown"))
		})

		It("should initialize state machine from internal state if available", func() {
			topology := &stormv1beta1.StormTopology{
				Status: stormv1beta1.StormTopologyStatus{
					InternalState: "Downloading",
					Phase:         "Pending",
				},
			}
			sm := reconciler.initializeStateMachine(topology)
			Expect(string(sm.CurrentState())).To(Equal("Downloading"))
		})

		It("should map phase to state when internal state is not available", func() {
			testCases := []struct {
				phase         string
				expectedState string
			}{
				{"Pending", "Pending"},
				{"Submitted", "Submitting"},
				{"Running", "Running"},
				{"Suspended", "Suspended"},
				{"Updating", "Updating"},
				{"Killed", "Killed"},
				{"Failed", "Failed"},
				{"Unknown", "Unknown"},
			}

			for _, tc := range testCases {
				topology := &stormv1beta1.StormTopology{
					Status: stormv1beta1.StormTopologyStatus{
						Phase: tc.phase,
					},
				}
				sm := reconciler.initializeStateMachine(topology)
				Expect(string(sm.CurrentState())).To(Equal(tc.expectedState))
			}
		})
	})

	Describe("State Transitions", func() {
		var topology *stormv1beta1.StormTopology
		var cluster *stormv1beta1.StormCluster
		var topologyCtx *TopologyContext

		BeforeEach(func() {
			// Create test objects
			cluster = &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "storm-system",
				},
				Status: stormv1beta1.StormClusterStatus{
					Phase: "Running",
				},
			}

			topology = &stormv1beta1.StormTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-topology",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormTopologySpec{
					ClusterRef: "test-cluster",
					Topology: stormv1beta1.TopologySpec{
						Name:      "test-topology",
						MainClass: "com.example.TestTopology",
						Jar: stormv1beta1.JarSpec{
							URL: "http://example.com/test.jar",
						},
						Config: map[string]string{
							"topology.version": "v1",
						},
					},
				},
			}

			// Create objects in fake client
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			Expect(fakeClient.Create(ctx, topology)).To(Succeed())

			topologyCtx = &TopologyContext{
				Topology:    topology,
				Cluster:     cluster,
				StormClient: mockStormClient,
			}
		})

		Context("From Unknown State", func() {
			It("should transition to Validating on Validate event", func() {
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("Validate"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Validating"))
			})
		})

		Context("From Validating State", func() {
			It("should transition to Downloading on successful validation", func() {
				topology.Status.InternalState = "Validating"
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("ValidationSuccess"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Downloading"))
			})

			It("should transition to Failed on validation failure", func() {
				topology.Status.InternalState = "Validating"
				topology.Spec.Topology.MainClass = "" // Invalid: no main class
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("ValidationFailed"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Failed"))
			})
		})

		Context("From Running State", func() {
			BeforeEach(func() {
				topology.Status.InternalState = "Running"
				topology.Status.DeployedVersion = "v1"
			})

			It("should transition to Suspended when suspend is requested", func() {
				topology.Spec.Suspend = true
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("Suspend"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Suspended"))
			})

			It("should transition to Updating when version changes", func() {
				topology.Spec.Topology.Config = map[string]string{
					"topology.version": "v2",
				}
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				// Mock GetTopology to return active topology
				mockStormClient.On("GetTopology", ctx, "test-topology").Return(&storm.TopologyInfo{
					Name:          "test-topology",
					Status:        "ACTIVE",
					Workers:       4,
					Executors:     8,
					Tasks:         16,
					UptimeSeconds: 3600,
				}, nil)

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("TopologyUpdate"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Updating"))
			})

			It("should stay in Running state when healthy", func() {
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				// Mock GetTopology to return active topology
				mockStormClient.On("GetTopology", ctx, "test-topology").Return(&storm.TopologyInfo{
					Name:          "test-topology",
					Status:        "ACTIVE",
					Workers:       4,
					Executors:     8,
					Tasks:         16,
					UptimeSeconds: 3600,
				}, nil)

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.TopologyEvent(""))) // No transition
			})

			It("should transition to Failed on health check error", func() {
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				// Mock GetTopology to return error
				mockStormClient.On("GetTopology", ctx, "test-topology").Return(nil, fmt.Errorf("topology not found"))

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("Error"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Failed"))
			})
		})

		Context("From Suspended State", func() {
			BeforeEach(func() {
				topology.Status.InternalState = "Suspended"
				topology.Spec.Suspend = true
			})

			It("should transition to Running when resume is requested", func() {
				topology.Spec.Suspend = false
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("Resume"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Running"))
			})

			It("should stay in Suspended state when still suspended", func() {
				sm := reconciler.initializeStateMachine(topology)
				topologyCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, topologyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.TopologyEvent(""))) // No transition
			})
		})
	})

	Describe("Reconciliation Flow", func() {
		var topology *stormv1beta1.StormTopology
		var cluster *stormv1beta1.StormCluster

		BeforeEach(func() {
			// Create test cluster
			cluster = &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "storm-system",
				},
				Status: stormv1beta1.StormClusterStatus{
					Phase: "Running",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())

			// Create test topology
			topology = &stormv1beta1.StormTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-topology",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormTopologySpec{
					ClusterRef: "test-cluster",
					Topology: stormv1beta1.TopologySpec{
						Name:      "test-topology",
						MainClass: "com.example.TestTopology",
						Jar: stormv1beta1.JarSpec{
							URL: "http://example.com/test.jar",
						},
						Config: map[string]string{
							"topology.version": "v1",
						},
					},
				},
			}
		})

		It("should handle topology creation and reconciliation", func() {
			// Create topology
			Expect(fakeClient.Create(ctx, topology)).To(Succeed())

			// Add finalizer (simulating what the reconciler would do)
			topology.Finalizers = []string{"storm.apache.org/topology-finalizer"}
			Expect(fakeClient.Update(ctx, topology)).To(Succeed())

			// Mock JAR download
			mockStormClient.On("DownloadJar", ctx, "http://example.com/test.jar").Return([]byte("jar-content"), nil)

			// First reconciliation - should validate
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      topology.Name,
					Namespace: topology.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// Check topology status
			updatedTopology := &stormv1beta1.StormTopology{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedTopology)).To(Succeed())
			Expect(updatedTopology.Status.InternalState).To(Equal("Validating"))
			Expect(updatedTopology.Status.Phase).To(Equal("Pending"))

			// Mock topology not found (not yet submitted)
			mockStormClient.On("GetTopology", ctx, "test-topology").Return(nil, fmt.Errorf("topology not found"))

			// Second reconciliation - should download and submit
			result, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Check topology status progressed
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedTopology)).To(Succeed())
			// State should have progressed from Validating
			Expect(updatedTopology.Status.InternalState).To(Or(Equal("Downloading"), Equal("Submitting")))

			// Mock successful submission
			mockStormClient.On("SubmitTopology", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// Mock topology now exists and is running
			mockStormClient.On("GetTopology", ctx, "test-topology").Return(&storm.TopologyInfo{
				Name:   "test-topology",
				Status: "ACTIVE",
			}, nil)

			// Third reconciliation - verify topology has been processed
			result, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Check that topology has progressed through states
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedTopology)).To(Succeed())
			// Should not be in Unknown or Failed state
			Expect(updatedTopology.Status.InternalState).NotTo(Equal("Unknown"))
			Expect(updatedTopology.Status.InternalState).NotTo(Equal("Failed"))
			// Phase should be set
			Expect(updatedTopology.Status.Phase).NotTo(BeEmpty())
		})

		It("should handle deletion with finalizer", func() {
			// Create topology with finalizer
			topology.Finalizers = []string{topologyFinalizer}
			Expect(fakeClient.Create(ctx, topology)).To(Succeed())

			// Mark for deletion
			Expect(fakeClient.Delete(ctx, topology)).To(Succeed())

			// Mock topology exists in Storm
			mockStormClient.On("KillTopology", ctx, "test-topology", 30).Return(nil)

			// Reconcile deletion
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      topology.Name,
					Namespace: topology.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Check finalizer removed
			updatedTopology := &stormv1beta1.StormTopology{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedTopology)
			Expect(err).To(HaveOccurred()) // Should be deleted
		})
	})

	Describe("Status Updates", func() {
		It("should map internal states to allowed phases correctly", func() {
			testCases := []struct {
				internalState string
				expectedPhase string
			}{
				{"Unknown", "Pending"},
				{"Pending", "Pending"},
				{"Validating", "Pending"},
				{"Downloading", "Pending"},
				{"Submitting", "Submitted"},
				{"Running", "Running"},
				{"Suspended", "Suspended"},
				{"Updating", "Updating"},
				{"Killing", "Killed"},
				{"Killed", "Killed"},
				{"Failed", "Failed"},
			}

			for _, tc := range testCases {
				phase := mapStateToPhase(state.State(tc.internalState))
				Expect(phase).To(Equal(tc.expectedPhase))
			}
		})

		It("should set correct requeue durations for different states", func() {
			testCases := []struct {
				state            string
				expectedDuration time.Duration
			}{
				{"Running", 60 * time.Second},
				{"Failed", 5 * time.Minute},
				{"Killed", 0},
				{"Pending", 10 * time.Second},
				{"Downloading", 10 * time.Second},
			}

			for _, tc := range testCases {
				duration := reconciler.getRequeueDuration(state.State(tc.state))
				Expect(duration).To(Equal(tc.expectedDuration))
			}
		})
	})
})

// Unit tests using standard testing package
func TestTopologyValidation(t *testing.T) {
	reconciler := &StormTopologyReconciler{}
	ctx := context.Background()

	tests := []struct {
		name        string
		topology    *stormv1beta1.StormTopology
		shouldError bool
	}{
		{
			name: "valid topology",
			topology: &stormv1beta1.StormTopology{
				Spec: stormv1beta1.StormTopologySpec{
					Topology: stormv1beta1.TopologySpec{
						Name:      "test",
						MainClass: "com.example.Test",
						Jar: stormv1beta1.JarSpec{
							URL: "http://example.com/test.jar",
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "missing JAR source",
			topology: &stormv1beta1.StormTopology{
				Spec: stormv1beta1.StormTopologySpec{
					Topology: stormv1beta1.TopologySpec{
						Name:      "test",
						MainClass: "com.example.Test",
						Jar:       stormv1beta1.JarSpec{},
					},
				},
			},
			shouldError: true,
		},
		{
			name: "missing main class",
			topology: &stormv1beta1.StormTopology{
				Spec: stormv1beta1.StormTopologySpec{
					Topology: stormv1beta1.TopologySpec{
						Name: "test",
						Jar: stormv1beta1.JarSpec{
							URL: "http://example.com/test.jar",
						},
					},
				},
			},
			shouldError: true,
		},
		{
			name: "missing topology name",
			topology: &stormv1beta1.StormTopology{
				Spec: stormv1beta1.StormTopologySpec{
					Topology: stormv1beta1.TopologySpec{
						MainClass: "com.example.Test",
						Jar: stormv1beta1.JarSpec{
							URL: "http://example.com/test.jar",
						},
					},
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topologyCtx := &TopologyContext{
				Topology: tt.topology,
			}
			err := reconciler.validateTopology(ctx, topologyCtx)
			if tt.shouldError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestBuildSubmitCommand(t *testing.T) {
	reconciler := &StormTopologyReconciler{}

	topology := &stormv1beta1.StormTopology{
		Spec: stormv1beta1.StormTopologySpec{
			Topology: stormv1beta1.TopologySpec{
				Name:      "test-topology",
				MainClass: "com.example.TestTopology",
				Args:      []string{"arg1", "arg2"},
				Config: map[string]string{
					"topology.workers":           "4",
					"topology.debug":             "false",
					"topology.max.spout.pending": "1000",
					"topology.name.suffix":       "prod",
				},
			},
		},
	}

	cluster := &stormv1beta1.StormCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "storm-system",
		},
	}

	cmd := reconciler.buildSubmitCommand(topology, cluster, "/tmp/test.jar")

	// Check basic command structure
	if len(cmd) < 6 {
		t.Errorf("expected at least 6 command parts, got %d", len(cmd))
	}

	// Check storm binary
	if cmd[0] != "/apache-storm/bin/storm" {
		t.Errorf("expected storm binary, got %s", cmd[0])
	}

	// Check jar command
	if cmd[1] != "jar" {
		t.Errorf("expected 'jar' command, got %s", cmd[1])
	}

	// Check jar path
	if cmd[2] != "/tmp/test.jar" {
		t.Errorf("expected jar path, got %s", cmd[2])
	}

	// Check main class
	if cmd[3] != "com.example.TestTopology" {
		t.Errorf("expected main class, got %s", cmd[3])
	}

	// Check topology name
	if cmd[4] != "test-topology" {
		t.Errorf("expected topology name, got %s", cmd[4])
	}

	// Check args
	if cmd[5] != "arg1" || cmd[6] != "arg2" {
		t.Errorf("expected args, got %v", cmd[5:7])
	}

	// Check nimbus configuration
	foundNimbus := false
	for i, arg := range cmd {
		if arg == "-c" && i+1 < len(cmd) {
			if cmd[i+1] == `nimbus.seeds=["test-cluster-nimbus.storm-system.svc.cluster.local"]` {
				foundNimbus = true
				break
			}
		}
	}
	if !foundNimbus {
		t.Error("expected nimbus configuration not found")
	}
}
