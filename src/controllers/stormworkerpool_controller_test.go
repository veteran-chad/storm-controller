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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/veteran-chad/storm-controller/pkg/state"
)

var _ = Describe("StormWorkerPool State Machine Controller", func() {
	var (
		reconciler      *StormWorkerPoolReconciler
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
			Build()

		// Create mocks
		mockStormClient = new(MockStormClient)
		mockClientMgr = new(MockClientManager)
		mockClientMgr.On("GetClient").Return(mockStormClient, nil)

		// Create reconciler
		reconciler = &StormWorkerPoolReconciler{
			Client:        fakeClient,
			Scheme:        scheme,
			ClientManager: mockClientMgr,
			Coordinator:   coordination.NewResourceCoordinator(fakeClient, mockClientMgr, scheme),
		}
	})

	Describe("State Machine Initialization", func() {
		It("should initialize state machine from Unknown state", func() {
			workerPool := &stormv1beta1.StormWorkerPool{
				Status: stormv1beta1.StormWorkerPoolStatus{
					Phase: "",
				},
			}
			sm := reconciler.initializeStateMachine(workerPool)
			Expect(string(sm.CurrentState())).To(Equal("Unknown"))
		})

		It("should map phase to state correctly", func() {
			testCases := []struct {
				phase         string
				expectedState string
			}{
				{"", "Unknown"},
				{"Pending", "Pending"},
				{"Creating", "Creating"},
				{"Running", "Ready"},
				{"Ready", "Ready"},
				{"Scaling", "Scaling"},
				{"Updating", "Updating"},
				{"Failed", "Failed"},
				{"Terminated", "Deleted"},
				{"Deleted", "Deleted"},
				{"Unknown", "Unknown"},
			}

			for _, tc := range testCases {
				workerPool := &stormv1beta1.StormWorkerPool{
					Status: stormv1beta1.StormWorkerPoolStatus{
						Phase: tc.phase,
					},
				}
				sm := reconciler.initializeStateMachine(workerPool)
				Expect(string(sm.CurrentState())).To(Equal(tc.expectedState))
			}
		})
	})

	Describe("State Transitions", func() {
		var workerPool *stormv1beta1.StormWorkerPool
		var topology *stormv1beta1.StormTopology
		var cluster *stormv1beta1.StormCluster
		var workerPoolCtx *WorkerPoolContext

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
				},
				Status: stormv1beta1.StormTopologyStatus{
					Phase: "Running",
				},
			}

			workerPool = &stormv1beta1.StormWorkerPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workerpool",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormWorkerPoolSpec{
					TopologyRef: "test-topology",
					ClusterRef:  "test-cluster",
					Replicas:    3,
				},
			}

			// Create objects in fake client
			Expect(fakeClient.Create(ctx, cluster)).To(Succeed())
			Expect(fakeClient.Create(ctx, topology)).To(Succeed())
			Expect(fakeClient.Create(ctx, workerPool)).To(Succeed())

			workerPoolCtx = &WorkerPoolContext{
				WorkerPool: workerPool,
				Topology:   topology,
				Cluster:    cluster,
			}
		})

		Context("From Unknown State", func() {
			It("should transition to Creating on WPCreate event", func() {
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("WPCreate"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Creating"))
			})
		})

		Context("From Pending State", func() {
			BeforeEach(func() {
				workerPool.Status.Phase = "Pending"
			})

			It("should wait if cluster is not ready", func() {
				cluster.Status.Phase = "Pending"
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.WorkerPoolEvent(""))) // No transition
			})

			It("should wait if topology is not ready", func() {
				topology.Status.Phase = "Pending"
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.WorkerPoolEvent(""))) // No transition
			})

			It("should transition to Creating when dependencies are ready", func() {
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("WPCreate"))
			})
		})

		Context("From Creating State", func() {
			BeforeEach(func() {
				workerPool.Status.Phase = "Creating"
			})

			It("should stay in Creating if resources not ready", func() {
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.WorkerPoolEvent(""))) // No transition
			})

			It("should transition to Ready when resources are created", func() {
				// Create deployment and service
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-workerpool-workers",
						Namespace: "storm-system",
					},
				}
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-workerpool-workers",
						Namespace: "storm-system",
					},
				}
				Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
				Expect(fakeClient.Create(ctx, service)).To(Succeed())

				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("WPCreateComplete"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Ready"))
			})
		})

		Context("From Ready State", func() {
			BeforeEach(func() {
				workerPool.Status.Phase = "Ready"
				workerPool.Status.DeploymentName = "test-workerpool-workers"

				// Create deployment
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-workerpool-workers",
						Namespace: "storm-system",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &[]int32{3}[0],
					},
					Status: appsv1.DeploymentStatus{
						Replicas:      3,
						ReadyReplicas: 3,
					},
				}
				Expect(fakeClient.Create(ctx, deployment)).To(Succeed())
			})

			It("should transition to Scaling when replicas change", func() {
				workerPool.Spec.Replicas = 5 // Changed from 3 to 5
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("ScaleUp"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Scaling"))
			})

			It("should transition to Failed on health check failure", func() {
				// Update deployment to unhealthy state
				deployment := &appsv1.Deployment{}
				Expect(fakeClient.Get(ctx, types.NamespacedName{
					Name:      "test-workerpool-workers",
					Namespace: "storm-system",
				}, deployment)).To(Succeed())

				deployment.Status.ReadyReplicas = 0
				Expect(fakeClient.Status().Update(ctx, deployment)).To(Succeed())

				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("HealthCheckFailed"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Failed"))
			})

			It("should stay in Ready state when healthy", func() {
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(event).To(Equal(state.WorkerPoolEvent(""))) // No transition
			})
		})

		Context("From Failed State", func() {
			BeforeEach(func() {
				workerPool.Status.Phase = "Failed"
			})

			It("should allow recovery", func() {
				sm := reconciler.initializeStateMachine(workerPool)
				workerPoolCtx.StateMachine = sm

				event, err := reconciler.determineNextEvent(ctx, workerPoolCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(event)).To(Equal("WPRecover"))

				err = sm.ProcessEvent(ctx, state.Event(event))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(sm.CurrentState())).To(Equal("Pending"))
			})
		})
	})

	Describe("Resource Creation", func() {
		var workerPool *stormv1beta1.StormWorkerPool
		var topology *stormv1beta1.StormTopology
		var cluster *stormv1beta1.StormCluster
		var workerPoolCtx *WorkerPoolContext

		BeforeEach(func() {
			cluster = &stormv1beta1.StormCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormClusterSpec{
					Image: &stormv1beta1.ImageSpec{
						Repository: "apache/storm",
						Tag:        "2.6.0",
					},
				},
			}

			topology = &stormv1beta1.StormTopology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-topology",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormTopologySpec{
					Topology: stormv1beta1.TopologySpec{
						Name: "test-topology",
					},
				},
			}

			workerPool = &stormv1beta1.StormWorkerPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workerpool",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormWorkerPoolSpec{
					TopologyRef: "test-topology",
					ClusterRef:  "test-cluster",
					Replicas:    3,
					Autoscaling: &stormv1beta1.AutoscalingSpec{
						Enabled:     true,
						MinReplicas: 2,
						MaxReplicas: 10,
					},
				},
			}

			workerPoolCtx = &WorkerPoolContext{
				WorkerPool: workerPool,
				Topology:   topology,
				Cluster:    cluster,
			}
		})

		It("should create deployment with correct spec", func() {
			err := reconciler.createWorkerPool(ctx, workerPoolCtx)
			Expect(err).NotTo(HaveOccurred())

			// Check deployment was created
			deployment := &appsv1.Deployment{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-workerpool-workers",
				Namespace: "storm-system",
			}, deployment)).To(Succeed())

			// Verify deployment spec
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
			Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue("workerpool", "test-workerpool"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("app", "storm"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("component", "worker"))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("topology", "test-topology"))

			// Verify container spec
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("worker"))
			Expect(container.Image).To(Equal("docker.io/apache/storm:2.6.0"))
			Expect(container.Command).To(Equal([]string{"storm", "supervisor"}))

			// Verify ports
			Expect(container.Ports).To(HaveLen(4)) // Default 4 ports
			Expect(container.Ports[0].ContainerPort).To(Equal(int32(6700)))
		})

		It("should create HPA when autoscaling is enabled", func() {
			err := reconciler.createWorkerPool(ctx, workerPoolCtx)
			Expect(err).NotTo(HaveOccurred())

			// Check HPA was created
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-workerpool-hpa",
				Namespace: "storm-system",
			}, hpa)).To(Succeed())

			// Verify HPA spec
			Expect(*hpa.Spec.MinReplicas).To(Equal(int32(2)))
			Expect(hpa.Spec.MaxReplicas).To(Equal(int32(10)))
			Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal("test-workerpool-workers"))
			Expect(hpa.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))
		})

		It("should create headless service", func() {
			err := reconciler.createWorkerPool(ctx, workerPoolCtx)
			Expect(err).NotTo(HaveOccurred())

			// Check service was created
			service := &corev1.Service{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-workerpool-workers",
				Namespace: "storm-system",
			}, service)).To(Succeed())

			// Verify service spec
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(service.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone)) // Headless
			Expect(service.Spec.Selector).To(HaveKeyWithValue("workerpool", "test-workerpool"))
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(6700)))
		})
	})

	Describe("Reconciliation Flow", func() {
		var workerPool *stormv1beta1.StormWorkerPool
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
				},
				Status: stormv1beta1.StormTopologyStatus{
					Phase: "Running",
				},
			}
			Expect(fakeClient.Create(ctx, topology)).To(Succeed())

			// Create test worker pool
			workerPool = &stormv1beta1.StormWorkerPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workerpool",
					Namespace: "storm-system",
				},
				Spec: stormv1beta1.StormWorkerPoolSpec{
					TopologyRef: "test-topology",
					ClusterRef:  "test-cluster",
					Replicas:    3,
				},
			}
		})

		It("should handle full lifecycle from creation to ready", func() {
			// Create worker pool
			Expect(fakeClient.Create(ctx, workerPool)).To(Succeed())

			// Add finalizer (simulating what the reconciler would do)
			workerPool.Finalizers = []string{"storm.apache.org/workerpool-finalizer"}
			Expect(fakeClient.Update(ctx, workerPool)).To(Succeed())

			// First reconciliation - should start creating
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// Check worker pool status
			updatedWorkerPool := &stormv1beta1.StormWorkerPool{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedWorkerPool)).To(Succeed())
			Expect(updatedWorkerPool.Status.Phase).To(Equal("Creating"))

			// Check deployment was created
			deployment := &appsv1.Deployment{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-workerpool-workers",
				Namespace: "storm-system",
			}, deployment)).To(Succeed())

			// Update deployment status to ready
			deployment.Status.Replicas = 3
			deployment.Status.ReadyReplicas = 3
			Expect(fakeClient.Status().Update(ctx, deployment)).To(Succeed())

			// Second reconciliation - should transition to ready
			result, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Check worker pool status
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedWorkerPool)).To(Succeed())
			Expect(updatedWorkerPool.Status.Phase).To(Equal("Ready"))
			Expect(updatedWorkerPool.Status.ReadyReplicas).To(Equal(int32(3)))
		})

		It("should handle deletion with finalizer", func() {
			// Create worker pool with finalizer
			workerPool.Finalizers = []string{workerPoolFinalizer}
			Expect(fakeClient.Create(ctx, workerPool)).To(Succeed())

			// Mark for deletion
			Expect(fakeClient.Delete(ctx, workerPool)).To(Succeed())

			// Reconcile deletion
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Check finalizer removed
			updatedWorkerPool := &stormv1beta1.StormWorkerPool{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedWorkerPool)
			Expect(err).To(HaveOccurred()) // Should be deleted
		})
	})

	Describe("Status Updates", func() {
		It("should set correct requeue durations for different states", func() {
			testCases := []struct {
				state            string
				expectedDuration time.Duration
			}{
				{"Ready", 60 * time.Second},
				{"Failed", 5 * time.Minute},
				{"Deleted", 0},
				{"Creating", 5 * time.Second},
				{"Scaling", 5 * time.Second},
				{"Updating", 5 * time.Second},
				{"Pending", 10 * time.Second},
			}

			for _, tc := range testCases {
				duration := reconciler.getRequeueDuration(state.State(tc.state))
				Expect(duration).To(Equal(tc.expectedDuration))
			}
		})

		It("should update component status from deployment", func() {
			workerPool := &stormv1beta1.StormWorkerPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workerpool",
					Namespace: "storm-system",
				},
				Status: stormv1beta1.StormWorkerPoolStatus{
					DeploymentName: "test-workerpool-workers",
				},
			}

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workerpool-workers",
					Namespace: "storm-system",
				},
				Status: appsv1.DeploymentStatus{
					Replicas:            5,
					ReadyReplicas:       4,
					AvailableReplicas:   4,
					UnavailableReplicas: 1,
					UpdatedReplicas:     5,
				},
			}
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			reconciler.updateComponentStatus(ctx, workerPool)

			Expect(workerPool.Status.Replicas).To(Equal(int32(5)))
			Expect(workerPool.Status.ReadyReplicas).To(Equal(int32(4)))
			Expect(workerPool.Status.AvailableReplicas).To(Equal(int32(4)))
			Expect(workerPool.Status.UnavailableReplicas).To(Equal(int32(1)))
			Expect(workerPool.Status.UpdatedReplicas).To(Equal(int32(5)))
		})
	})
})

// Unit tests using standard testing package
func TestWorkerPoolResourceChecks(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = stormv1beta1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &StormWorkerPoolReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	t.Run("checkResourcesCreated", func(t *testing.T) {
		workerPool := &stormv1beta1.StormWorkerPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workerpool",
				Namespace: "default",
			},
		}

		// Should fail when resources don't exist
		err := reconciler.checkResourcesCreated(ctx, workerPool)
		if err == nil {
			t.Error("expected error when resources don't exist")
		}

		// Create deployment
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workerpool-workers",
				Namespace: "default",
			},
		}
		if err := fakeClient.Create(ctx, deployment); err != nil {
			t.Fatalf("failed to create deployment: %v", err)
		}

		// Should still fail without service
		err = reconciler.checkResourcesCreated(ctx, workerPool)
		if err == nil {
			t.Error("expected error when service doesn't exist")
		}

		// Create service
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workerpool-workers",
				Namespace: "default",
			},
		}
		if err := fakeClient.Create(ctx, service); err != nil {
			t.Fatalf("failed to create service: %v", err)
		}

		// Should succeed with both resources
		err = reconciler.checkResourcesCreated(ctx, workerPool)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("checkScalingComplete", func(t *testing.T) {
		workerPool := &stormv1beta1.StormWorkerPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workerpool-scaling",
				Namespace: "default",
			},
			Status: stormv1beta1.StormWorkerPoolStatus{
				DeploymentName: "test-workerpool-scaling-workers",
			},
		}

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-workerpool-scaling-workers",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &[]int32{5}[0],
			},
			Status: appsv1.DeploymentStatus{
				ReadyReplicas: 3, // Not all ready
			},
		}
		if err := fakeClient.Create(ctx, deployment); err != nil {
			t.Fatalf("failed to create deployment: %v", err)
		}

		// Should return error when not all replicas are ready
		err := reconciler.checkScalingComplete(ctx, workerPool)
		if err == nil {
			t.Error("expected error when scaling is not complete")
		}

		// Update deployment to all replicas ready
		deployment.Status.ReadyReplicas = 5
		if err := fakeClient.Status().Update(ctx, deployment); err != nil {
			t.Fatalf("failed to update deployment: %v", err)
		}

		// Should succeed when all replicas are ready
		err = reconciler.checkScalingComplete(ctx, workerPool)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestBuildWorkerDeploymentSpec(t *testing.T) {
	reconciler := &StormWorkerPoolReconciler{}

	workerPool := &stormv1beta1.StormWorkerPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-workerpool",
		},
		Spec: stormv1beta1.StormWorkerPoolSpec{
			Replicas: 5,
			Ports: &stormv1beta1.PortConfig{
				Start: 7000,
				Count: 2,
			},
		},
	}

	topology := &stormv1beta1.StormTopology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-topology",
		},
		Spec: stormv1beta1.StormTopologySpec{
			Topology: stormv1beta1.TopologySpec{
				Name: "test-topology",
			},
		},
	}

	cluster := &stormv1beta1.StormCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: stormv1beta1.StormClusterSpec{
			Image: &stormv1beta1.ImageSpec{
				Registry:   "custom-registry.io",
				Repository: "storm/custom",
				Tag:        "1.2.3",
			},
		},
	}

	spec := reconciler.buildWorkerDeploymentSpec(workerPool, topology, cluster)

	// Check replicas
	if *spec.Replicas != 5 {
		t.Errorf("expected 5 replicas, got %d", *spec.Replicas)
	}

	// Check selector
	if spec.Selector.MatchLabels["workerpool"] != "test-workerpool" {
		t.Error("expected workerpool selector")
	}

	// Check pod template labels
	expectedLabels := map[string]string{
		"app":        "storm",
		"component":  "worker",
		"cluster":    "test-cluster",
		"topology":   "test-topology",
		"workerpool": "test-workerpool",
	}
	for k, v := range expectedLabels {
		if spec.Template.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, spec.Template.Labels[k])
		}
	}

	// Check container
	if len(spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Template.Spec.Containers))
	}
	container := spec.Template.Spec.Containers[0]

	// Check image
	expectedImage := "custom-registry.io/storm/custom:1.2.3"
	if container.Image != expectedImage {
		t.Errorf("expected image %s, got %s", expectedImage, container.Image)
	}

	// Check ports
	if len(container.Ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 7000 {
		t.Errorf("expected port 7000, got %d", container.Ports[0].ContainerPort)
	}
	if container.Ports[1].ContainerPort != 7001 {
		t.Errorf("expected port 7001, got %d", container.Ports[1].ContainerPort)
	}
}

func TestBuildHPASpec(t *testing.T) {
	reconciler := &StormWorkerPoolReconciler{}

	workerPool := &stormv1beta1.StormWorkerPool{
		Spec: stormv1beta1.StormWorkerPoolSpec{
			Autoscaling: &stormv1beta1.AutoscalingSpec{
				MinReplicas: 3,
				MaxReplicas: 15,
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
	}

	spec := reconciler.buildHPASpec(workerPool, deployment)

	// Check scale target
	if spec.ScaleTargetRef.Name != "test-deployment" {
		t.Errorf("expected target deployment name %s, got %s", "test-deployment", spec.ScaleTargetRef.Name)
	}
	if spec.ScaleTargetRef.Kind != "Deployment" {
		t.Errorf("expected target kind Deployment, got %s", spec.ScaleTargetRef.Kind)
	}

	// Check min/max replicas
	if *spec.MinReplicas != 3 {
		t.Errorf("expected min replicas 3, got %d", *spec.MinReplicas)
	}
	if spec.MaxReplicas != 15 {
		t.Errorf("expected max replicas 15, got %d", spec.MaxReplicas)
	}

	// Check metrics
	if len(spec.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(spec.Metrics))
	}
	if spec.Metrics[0].Type != autoscalingv2.ResourceMetricSourceType {
		t.Error("expected resource metric type")
	}
	if spec.Metrics[0].Resource.Name != corev1.ResourceCPU {
		t.Error("expected CPU metric")
	}
}
