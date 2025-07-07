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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/coordination"
	"github.com/veteran-chad/storm-controller/pkg/metrics"
	"github.com/veteran-chad/storm-controller/pkg/state"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// StormWorkerPoolReconcilerStateMachine reconciles a StormWorkerPool object using state machines
type StormWorkerPoolReconcilerStateMachine struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientManager storm.ClientManager
	Coordinator   *coordination.ResourceCoordinator
}

// WorkerPoolContext holds the context for worker pool reconciliation
type WorkerPoolContext struct {
	WorkerPool   *stormv1beta1.StormWorkerPool
	Topology     *stormv1beta1.StormTopology
	Cluster      *stormv1beta1.StormCluster
	StateMachine *state.StateMachine
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/finalizers,verbs=update
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies,verbs=get;list;watch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile handles StormWorkerPool reconciliation using state machines
func (r *StormWorkerPoolReconcilerStateMachine) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormWorkerPool instance
	workerPool := &stormv1beta1.StormWorkerPool{}
	if err := r.Get(ctx, req.NamespacedName, workerPool); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if workerPool.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, workerPool)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(workerPool, workerPoolFinalizer) {
		controllerutil.AddFinalizer(workerPool, workerPoolFinalizer)
		if err := r.Update(ctx, workerPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get referenced topology
	topology := &stormv1beta1.StormTopology{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      workerPool.Spec.TopologyRef,
		Namespace: workerPool.Namespace,
	}, topology); err != nil {
		log.Error(err, "Failed to get referenced StormTopology")
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("topology %s not found", workerPool.Spec.TopologyRef))
	}

	// Get referenced cluster
	cluster := &stormv1beta1.StormCluster{}
	clusterRef := workerPool.Spec.ClusterRef
	if clusterRef == "" {
		clusterRef = topology.Spec.ClusterRef
	}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      clusterRef,
		Namespace: workerPool.Namespace,
	}, cluster); err != nil {
		log.Error(err, "Failed to get referenced StormCluster")
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("cluster %s not found", clusterRef))
	}

	// Create worker pool context
	workerPoolCtx := &WorkerPoolContext{
		WorkerPool: workerPool,
		Topology:   topology,
		Cluster:    cluster,
	}

	// Initialize state machine
	sm := r.initializeStateMachine(workerPool)
	workerPoolCtx.StateMachine = sm

	// Process the worker pool based on its current state
	event, err := r.determineNextEvent(ctx, workerPoolCtx)
	if err != nil {
		log.Error(err, "Failed to determine next event")
		return ctrl.Result{}, err
	}

	if event != "" {
		log.Info("Processing event", "event", event, "currentState", sm.CurrentState())

		if err := sm.ProcessEvent(ctx, state.Event(event)); err != nil {
			log.Error(err, "Failed to process event", "event", event)
			return ctrl.Result{}, r.updateWorkerPoolStatus(ctx, workerPool, sm.CurrentState(), err.Error())
		}

		// Execute action for the new state
		if err := r.executeStateAction(ctx, workerPoolCtx); err != nil {
			log.Error(err, "Failed to execute state action", "state", sm.CurrentState())
			return ctrl.Result{}, r.updateWorkerPoolStatus(ctx, workerPool, sm.CurrentState(), err.Error())
		}

		// Update status with new state
		if err := r.updateWorkerPoolStatus(ctx, workerPool, sm.CurrentState(), ""); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Determine requeue time based on state
	requeueAfter := r.getRequeueDuration(sm.CurrentState())
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// initializeStateMachine creates and initializes a state machine based on worker pool status
func (r *StormWorkerPoolReconcilerStateMachine) initializeStateMachine(workerPool *stormv1beta1.StormWorkerPool) *state.StateMachine {
	// Map worker pool status phase to state machine state
	var currentState state.State
	switch workerPool.Status.Phase {
	case "Pending":
		currentState = state.State(state.WorkerPoolStatePending)
	case "Creating":
		currentState = state.State(state.WorkerPoolStateCreating)
	case "Running", "Ready":
		currentState = state.State(state.WorkerPoolStateReady)
	case "Scaling":
		currentState = state.State(state.WorkerPoolStateScaling)
	case "Updating":
		currentState = state.State(state.WorkerPoolStateUpdating)
	case "Failed":
		currentState = state.State(state.WorkerPoolStateFailed)
	case "Terminated", "Deleted":
		currentState = state.State(state.WorkerPoolStateDeleted)
	default:
		currentState = state.State(state.WorkerPoolStateUnknown)
	}

	// Create state machine
	sm := state.NewWorkerPoolStateMachine()

	// If we have a known state, recreate the state machine with that state
	if currentState != state.State(state.WorkerPoolStateUnknown) {
		sm = state.NewStateMachine(currentState)
		// Re-add all worker pool transitions
		r.setupWorkerPoolTransitions(sm)
	}

	// Set up handlers
	r.setupWorkerPoolHandlers(sm)

	return sm
}

// setupWorkerPoolTransitions sets up all state transitions
func (r *StormWorkerPoolReconcilerStateMachine) setupWorkerPoolTransitions(sm *state.StateMachine) {
	// Copy transitions from the standard worker pool state machine
	workerPoolSM := state.NewWorkerPoolStateMachine()
	*sm = *workerPoolSM
}

// setupWorkerPoolHandlers sets up state handlers
func (r *StormWorkerPoolReconcilerStateMachine) setupWorkerPoolHandlers(sm *state.StateMachine) {
	sm.SetTransitionFunc(func(ctx context.Context, from, to state.State, event state.Event) error {
		log := log.FromContext(ctx)
		log.Info("WorkerPool state transition", "from", from, "to", to, "event", event)
		return nil
	})
}

// determineNextEvent determines the next event based on current state
func (r *StormWorkerPoolReconcilerStateMachine) determineNextEvent(ctx context.Context, workerPoolCtx *WorkerPoolContext) (state.WorkerPoolEvent, error) {
	log := log.FromContext(ctx)
	workerPool := workerPoolCtx.WorkerPool
	topology := workerPoolCtx.Topology
	cluster := workerPoolCtx.Cluster
	currentState := workerPoolCtx.StateMachine.CurrentState()

	log.Info("Determining next event", "currentState", currentState)

	switch state.WorkerPoolState(currentState) {
	case state.WorkerPoolStateUnknown:
		return state.EventWPCreate, nil

	case state.WorkerPoolStatePending:
		// Check dependencies
		if cluster.Status.Phase != "Running" {
			return "", nil // Wait for cluster
		}
		if topology.Status.Phase != "Running" {
			return "", nil // Wait for topology
		}
		return state.EventWPCreate, nil

	case state.WorkerPoolStateCreating:
		// Check if resources are created
		if err := r.checkResourcesCreated(ctx, workerPool); err != nil {
			return "", nil // Still creating
		}
		return state.EventWPCreateComplete, nil

	case state.WorkerPoolStateReady:
		// Check if scaling is needed
		if r.needsScaling(ctx, workerPool) {
			return state.EventScaleUp, nil
		}
		// Check if update is needed
		if r.needsUpdate(ctx, workerPoolCtx) {
			return state.EventWPUpdateConfig, nil
		}
		// Check health
		if err := r.checkWorkerPoolHealth(ctx, workerPool); err != nil {
			return state.EventWPCreateFailed, nil
		}
		return "", nil // Stay in ready

	case state.WorkerPoolStateScaling:
		// Check if scaling is complete
		if err := r.checkScalingComplete(ctx, workerPool); err != nil {
			return "", nil // Still scaling
		}
		return state.EventWPScaleComplete, nil

	case state.WorkerPoolStateUpdating:
		// Check if update is complete
		if err := r.checkUpdateComplete(ctx, workerPool); err != nil {
			return "", nil // Still updating
		}
		return state.EventWPUpdateComplete, nil

	case state.WorkerPoolStateFailed:
		// Check if recovery is possible
		if r.canRecover(workerPool) {
			return state.EventWPRecover, nil
		}
		return "", nil // Stay in failed

	default:
		return "", nil
	}
}

// executeStateAction executes actions for the current state
func (r *StormWorkerPoolReconcilerStateMachine) executeStateAction(ctx context.Context, workerPoolCtx *WorkerPoolContext) error {
	currentState := workerPoolCtx.StateMachine.CurrentState()
	workerPool := workerPoolCtx.WorkerPool

	switch state.WorkerPoolState(currentState) {
	case state.WorkerPoolStateCreating:
		return r.createWorkerPool(ctx, workerPoolCtx)

	case state.WorkerPoolStateScaling:
		return r.scaleWorkerPool(ctx, workerPool)

	case state.WorkerPoolStateUpdating:
		return r.updateWorkerPool(ctx, workerPoolCtx)

	case state.WorkerPoolStateDeleting:
		return r.terminateWorkerPool(ctx, workerPool)

	default:
		// No action needed for other states
		return nil
	}
}

// createWorkerPool creates worker pool resources
func (r *StormWorkerPoolReconcilerStateMachine) createWorkerPool(ctx context.Context, workerPoolCtx *WorkerPoolContext) error {
	log := log.FromContext(ctx)
	workerPool := workerPoolCtx.WorkerPool
	topology := workerPoolCtx.Topology
	cluster := workerPoolCtx.Cluster

	log.Info("Creating worker pool", "workerpool", workerPool.Name)

	// Create deployment
	deployment, err := r.reconcileDeployment(ctx, workerPool, topology, cluster)
	if err != nil {
		return fmt.Errorf("failed to reconcile deployment: %w", err)
	}

	// Setup HPA if autoscaling is enabled
	if workerPool.Spec.Autoscaling != nil && workerPool.Spec.Autoscaling.Enabled {
		if err := r.reconcileHPA(ctx, workerPool, deployment); err != nil {
			return fmt.Errorf("failed to reconcile HPA: %w", err)
		}
	}

	// Create headless service for worker discovery
	if err := r.reconcileService(ctx, workerPool); err != nil {
		return fmt.Errorf("failed to reconcile service: %w", err)
	}

	return nil
}

// scaleWorkerPool handles scaling operations
func (r *StormWorkerPoolReconcilerStateMachine) scaleWorkerPool(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	log := log.FromContext(ctx)
	log.Info("Scaling worker pool", "workerpool", workerPool.Name)

	// Update deployment replicas
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return err
	}

	if deployment.Spec.Replicas != &workerPool.Spec.Replicas {
		deployment.Spec.Replicas = &workerPool.Spec.Replicas
		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

// updateWorkerPool handles updates
func (r *StormWorkerPoolReconcilerStateMachine) updateWorkerPool(ctx context.Context, workerPoolCtx *WorkerPoolContext) error {
	log := log.FromContext(ctx)
	log.Info("Updating worker pool", "workerpool", workerPoolCtx.WorkerPool.Name)

	// Recreate deployment with new configuration
	return r.createWorkerPool(ctx, workerPoolCtx)
}

// terminateWorkerPool handles termination
func (r *StormWorkerPoolReconcilerStateMachine) terminateWorkerPool(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	// Resources will be garbage collected due to owner references
	return nil
}

// Helper methods for state checking

// checkResourcesCreated checks if all resources are created
func (r *StormWorkerPoolReconcilerStateMachine) checkResourcesCreated(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	// Check if deployment exists
	deployment := &appsv1.Deployment{}
	deploymentName := fmt.Sprintf("%s-workers", workerPool.Name)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      deploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return err
	}

	// Check if service exists
	service := &corev1.Service{}
	serviceName := fmt.Sprintf("%s-workers", workerPool.Name)
	if err := r.Get(ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: workerPool.Namespace,
	}, service); err != nil {
		return err
	}

	return nil
}

// needsScaling checks if worker pool needs scaling
func (r *StormWorkerPoolReconcilerStateMachine) needsScaling(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) bool {
	// Check if current replicas differ from desired
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return false
	}

	return deployment.Spec.Replicas != &workerPool.Spec.Replicas
}

// needsUpdate checks if worker pool needs update
func (r *StormWorkerPoolReconcilerStateMachine) needsUpdate(ctx context.Context, workerPoolCtx *WorkerPoolContext) bool {
	// Check if spec has changed
	// For now, always return false as we don't track ObservedGeneration
	return false
}

// checkWorkerPoolHealth checks if worker pool is healthy
func (r *StormWorkerPoolReconcilerStateMachine) checkWorkerPoolHealth(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return err
	}

	if deployment.Status.ReadyReplicas < *deployment.Spec.Replicas/2 {
		return fmt.Errorf("less than half of workers are ready")
	}

	return nil
}

// checkScalingComplete checks if scaling is complete
func (r *StormWorkerPoolReconcilerStateMachine) checkScalingComplete(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return err
	}

	if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
		return fmt.Errorf("scaling in progress")
	}

	return nil
}

// checkUpdateComplete checks if update is complete
func (r *StormWorkerPoolReconcilerStateMachine) checkUpdateComplete(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return err
	}

	if deployment.Status.UpdatedReplicas != *deployment.Spec.Replicas {
		return fmt.Errorf("update in progress")
	}

	return nil
}

// canRecover checks if worker pool can recover from failed state
func (r *StormWorkerPoolReconcilerStateMachine) canRecover(workerPool *stormv1beta1.StormWorkerPool) bool {
	// For now, always allow recovery attempts
	return true
}

// updateWorkerPoolStatus updates the worker pool status based on state
func (r *StormWorkerPoolReconcilerStateMachine) updateWorkerPoolStatus(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, currentState state.State, errorMsg string) error {
	workerPool.Status.Phase = string(currentState)
	workerPool.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Update ready condition
	if errorMsg != "" {
		workerPool.Status.Message = errorMsg
		meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: workerPool.Generation,
			Reason:             string(currentState),
			Message:            errorMsg,
		})
	} else {
		readyStates := map[state.State]bool{
			state.State(state.WorkerPoolStateReady): true,
		}

		conditionStatus := metav1.ConditionFalse
		if readyStates[currentState] {
			conditionStatus = metav1.ConditionTrue
		}

		meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             conditionStatus,
			ObservedGeneration: workerPool.Generation,
			Reason:             string(currentState),
			Message:            fmt.Sprintf("WorkerPool is in %s state", currentState),
		})
	}

	// Update component status if deployment exists
	r.updateComponentStatus(ctx, workerPool)

	return r.Status().Update(ctx, workerPool)
}

// updateComponentStatus updates component status counts
func (r *StormWorkerPoolReconcilerStateMachine) updateComponentStatus(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) {
	if workerPool.Status.DeploymentName == "" {
		return
	}

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		return
	}

	// Update replica counts
	workerPool.Status.Replicas = deployment.Status.Replicas
	workerPool.Status.ReadyReplicas = deployment.Status.ReadyReplicas
	workerPool.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	workerPool.Status.UnavailableReplicas = deployment.Status.UnavailableReplicas
	workerPool.Status.UpdatedReplicas = deployment.Status.UpdatedReplicas

	// Update metrics
	metrics.StormWorkerPoolReplicas.With(map[string]string{
		"pool":      workerPool.Name,
		"namespace": workerPool.Namespace,
		"topology":  workerPool.Spec.TopologyRef,
		"state":     "desired",
	}).Set(float64(workerPool.Spec.Replicas))

	metrics.StormWorkerPoolReplicas.With(map[string]string{
		"pool":      workerPool.Name,
		"namespace": workerPool.Namespace,
		"topology":  workerPool.Spec.TopologyRef,
		"state":     "ready",
	}).Set(float64(workerPool.Status.ReadyReplicas))
}

// getRequeueDuration returns the requeue duration based on state
func (r *StormWorkerPoolReconcilerStateMachine) getRequeueDuration(currentState state.State) time.Duration {
	switch state.WorkerPoolState(currentState) {
	case state.WorkerPoolStateReady:
		return 60 * time.Second // Check every minute when ready
	case state.WorkerPoolStateFailed:
		return 5 * time.Minute // Check less frequently when failed
	case state.WorkerPoolStateDeleted:
		return 0 // Don't requeue terminal states
	case state.WorkerPoolStateCreating, state.WorkerPoolStateScaling, state.WorkerPoolStateUpdating:
		return 5 * time.Second // Check frequently during transitions
	default:
		return 10 * time.Second // Default requeue
	}
}

// updateStatusError updates the status with an error
func (r *StormWorkerPoolReconcilerStateMachine) updateStatusError(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, err error) (ctrl.Result, error) {
	if statusErr := r.updateWorkerPoolStatus(ctx, workerPool, state.State(state.WorkerPoolStateFailed), err.Error()); statusErr != nil {
		return ctrl.Result{}, statusErr
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleDeletion handles worker pool deletion
func (r *StormWorkerPoolReconcilerStateMachine) handleDeletion(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(workerPool, workerPoolFinalizer) {
		log.Info("Handling worker pool deletion", "workerpool", workerPool.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(workerPool, workerPoolFinalizer)
		if err := r.Update(ctx, workerPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Resource reconciliation methods (imported from enhanced controller)

// reconcileDeployment creates or updates the worker deployment
func (r *StormWorkerPoolReconcilerStateMachine) reconcileDeployment(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) (*appsv1.Deployment, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("%s-workers", workerPool.Name)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, deployment, r.Scheme); err != nil {
			return err
		}

		// Build deployment spec
		deployment.Spec = r.buildWorkerDeploymentSpec(workerPool, topology, cluster)

		// Update deployment name in status
		workerPool.Status.DeploymentName = deploymentName

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Info("Reconciled worker deployment", "name", deployment.Name)
	return deployment, nil
}

// buildWorkerDeploymentSpec builds the worker deployment specification
func (r *StormWorkerPoolReconcilerStateMachine) buildWorkerDeploymentSpec(workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := workerPool.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	labels := map[string]string{
		"app":        "storm",
		"component":  "worker",
		"cluster":    cluster.Name,
		"topology":   topology.Name,
		"workerpool": workerPool.Name,
	}

	// Build container
	container := r.buildWorkerContainer(workerPool, topology, cluster)

	// Build pod spec
	podSpec := r.buildWorkerPodSpec(workerPool, cluster, container)

	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"workerpool": workerPool.Name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: *podSpec,
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
	}
}

// buildWorkerContainer builds the worker container specification
func (r *StormWorkerPoolReconcilerStateMachine) buildWorkerContainer(workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) corev1.Container {
	// Determine image
	image := r.getStormImage(cluster)
	if workerPool.Spec.Image != nil {
		image = r.getImageFromSpec(workerPool.Spec.Image)
	}

	// Build ports
	ports := []corev1.ContainerPort{}
	portStart := int32(6700)
	portCount := int32(4)

	if workerPool.Spec.Ports != nil {
		if workerPool.Spec.Ports.Start > 0 {
			portStart = workerPool.Spec.Ports.Start
		}
		if workerPool.Spec.Ports.Count > 0 {
			portCount = workerPool.Spec.Ports.Count
		}
	}

	for i := int32(0); i < portCount; i++ {
		ports = append(ports, corev1.ContainerPort{
			Name:          fmt.Sprintf("worker-%d", i),
			ContainerPort: portStart + i,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Default resources
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	}

	return corev1.Container{
		Name:      "worker",
		Image:     image,
		Command:   []string{"storm", "supervisor"},
		Ports:     ports,
		Resources: resources,
		Env: []corev1.EnvVar{
			{
				Name:  "STORM_CONF_DIR",
				Value: "/conf",
			},
			{
				Name:  "TOPOLOGY_NAME",
				Value: topology.Spec.Topology.Name,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
		},
	}
}

// buildWorkerPodSpec builds the worker pod specification
func (r *StormWorkerPoolReconcilerStateMachine) buildWorkerPodSpec(workerPool *stormv1beta1.StormWorkerPool, cluster *stormv1beta1.StormCluster, container corev1.Container) *corev1.PodSpec {
	// Determine configmap name based on cluster's management mode
	configMapName := stormConfigName
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.ConfigMap != "" {
		configMapName = cluster.Spec.ResourceNames.ConfigMap
	}

	return &corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: r.getImagePullSecrets(cluster),
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			},
		},
	}
}

// reconcileHPA creates or updates the HorizontalPodAutoscaler
func (r *StormWorkerPoolReconcilerStateMachine) reconcileHPA(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, deployment *appsv1.Deployment) error {
	log := log.FromContext(ctx)

	hpaName := fmt.Sprintf("%s-hpa", workerPool.Name)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, hpa, r.Scheme); err != nil {
			return err
		}

		// Build HPA spec
		hpa.Spec = r.buildHPASpec(workerPool, deployment)
		workerPool.Status.HPAName = hpaName

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled HPA", "name", hpa.Name)
	return nil
}

// buildHPASpec builds the HPA specification
func (r *StormWorkerPoolReconcilerStateMachine) buildHPASpec(workerPool *stormv1beta1.StormWorkerPool, deployment *appsv1.Deployment) autoscalingv2.HorizontalPodAutoscalerSpec {
	minReplicas := workerPool.Spec.Autoscaling.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}

	maxReplicas := workerPool.Spec.Autoscaling.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = 10
	}

	return autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
		},
		MinReplicas: &minReplicas,
		MaxReplicas: maxReplicas,
		Metrics: []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &[]int32{80}[0], // Default 80% CPU
					},
				},
			},
		},
	}
}

// reconcileService creates or updates the headless service for worker discovery
func (r *StormWorkerPoolReconcilerStateMachine) reconcileService(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	log := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-workers", workerPool.Name),
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, service, r.Scheme); err != nil {
			return err
		}

		// Build service spec
		service.Spec = corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone, // Headless service
			Selector: map[string]string{
				"workerpool": workerPool.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "worker-6700",
					Port:       6700,
					TargetPort: intstr.FromInt(6700),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled worker service", "name", service.Name)
	return nil
}

// Helper functions

func (r *StormWorkerPoolReconcilerStateMachine) getStormImage(cluster *stormv1beta1.StormCluster) string {
	registry := cluster.Spec.Image.Registry
	if registry == "" {
		registry = "docker.io"
	}
	repository := cluster.Spec.Image.Repository
	if repository == "" {
		repository = "apache/storm"
	}
	tag := cluster.Spec.Image.Tag
	if tag == "" {
		tag = "2.6.0"
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func (r *StormWorkerPoolReconcilerStateMachine) getImageFromSpec(imageSpec *stormv1beta1.ImageSpec) string {
	registry := imageSpec.Registry
	if registry == "" {
		registry = "docker.io"
	}
	repository := imageSpec.Repository
	if repository == "" {
		repository = "apache/storm"
	}
	tag := imageSpec.Tag
	if tag == "" {
		tag = "2.6.0"
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func (r *StormWorkerPoolReconcilerStateMachine) getImagePullSecrets(cluster *stormv1beta1.StormCluster) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	for _, secret := range cluster.Spec.Image.PullSecrets {
		secrets = append(secrets, corev1.LocalObjectReference{Name: secret})
	}
	return secrets
}

// SetupWithManager sets up the controller with the Manager
func (r *StormWorkerPoolReconcilerStateMachine) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormWorkerPool{}).
		Owns(&appsv1.Deployment{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&corev1.Service{}).
		Watches(&stormv1beta1.StormTopology{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkerPoolsForTopology)).
		Watches(&stormv1beta1.StormCluster{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkerPoolsForCluster)).
		Complete(r)
}

// Cross-reference methods (copied from enhanced controller for completeness)

func (r *StormWorkerPoolReconcilerStateMachine) findWorkerPoolsForCluster(ctx context.Context, obj client.Object) []ctrl.Request {
	cluster := obj.(*stormv1beta1.StormCluster)

	workerPoolList := &stormv1beta1.StormWorkerPoolList{}
	if err := r.List(ctx, workerPoolList, client.InNamespace(cluster.Namespace)); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, workerPool := range workerPoolList.Items {
		if workerPool.Spec.ClusterRef == cluster.Name {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			})
			continue
		}

		if workerPool.Spec.TopologyRef != "" {
			topology := &stormv1beta1.StormTopology{}
			if err := r.Get(ctx, client.ObjectKey{
				Name:      workerPool.Spec.TopologyRef,
				Namespace: workerPool.Namespace,
			}, topology); err == nil && topology.Spec.ClusterRef == cluster.Name {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      workerPool.Name,
						Namespace: workerPool.Namespace,
					},
				})
			}
		}
	}

	return requests
}

func (r *StormWorkerPoolReconcilerStateMachine) findWorkerPoolsForTopology(ctx context.Context, obj client.Object) []ctrl.Request {
	topology := obj.(*stormv1beta1.StormTopology)

	workerPoolList := &stormv1beta1.StormWorkerPoolList{}
	if err := r.List(ctx, workerPoolList, client.InNamespace(topology.Namespace)); err != nil {
		return nil
	}

	requests := []ctrl.Request{}
	for _, workerPool := range workerPoolList.Items {
		if workerPool.Spec.TopologyRef == topology.Name {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			})
		}
	}

	return requests
}
