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
	"regexp"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/config"
	"github.com/veteran-chad/storm-controller/pkg/coordination"
	"github.com/veteran-chad/storm-controller/pkg/state"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

const (
	clusterFinalizer = "storm.apache.org/stormcluster-finalizer"
)

// StormClusterReconciler reconciles a StormCluster object using state machines
type StormClusterReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ClientManager     storm.ClientManager
	Coordinator       *coordination.ResourceCoordinator
	OperatorNamespace string
	OperatorConfig    *config.OperatorConfig
}

// ClusterContext holds the context for cluster reconciliation
type ClusterContext struct {
	Cluster      *stormv1beta1.StormCluster
	StateMachine *state.StateMachine
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/finalizers,verbs=update

// Reconcile handles StormCluster reconciliation using state machines
func (r *StormClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormCluster instance
	cluster := &stormv1beta1.StormCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if cluster.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, cluster)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		controllerutil.AddFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Load operator config if not already loaded
	if r.OperatorConfig == nil {
		cfg, err := config.LoadOperatorConfig(ctx, r.Client, r.OperatorNamespace)
		if err != nil {
			log.Error(err, "Failed to load operator config, using defaults")
			// Fall back to default config
			r.OperatorConfig = config.GetDefaultConfig()
		} else {
			r.OperatorConfig = cfg
		}
	}

	// Apply defaults from operator config to the cluster
	config.ApplyDefaults(cluster, r.OperatorConfig)

	// Create cluster context
	clusterCtx := &ClusterContext{
		Cluster: cluster,
	}

	// Initialize state machine
	sm := r.initializeStateMachine(cluster)
	clusterCtx.StateMachine = sm

	// Process the cluster based on its current state
	event, err := r.determineNextEvent(ctx, clusterCtx)
	if err != nil {
		log.Error(err, "Failed to determine next event")
		return ctrl.Result{}, err
	}

	if event != "" {
		log.Info("Processing event", "event", event, "currentState", sm.CurrentState())

		if err := sm.ProcessEvent(ctx, state.Event(event)); err != nil {
			log.Error(err, "Failed to process event", "event", event)
			return ctrl.Result{}, r.updateClusterStatus(ctx, cluster, sm.CurrentState(), err.Error())
		}

		// Execute action for the new state (only for states that need actions)
		needsAction := false
		switch state.ClusterState(sm.CurrentState()) {
		case state.ClusterStateCreating, state.ClusterStateUpdating, state.ClusterStateTerminating:
			needsAction = true
		}

		if needsAction {
			if err := r.executeStateAction(ctx, clusterCtx); err != nil {
				log.Error(err, "Failed to execute state action", "state", sm.CurrentState())
				return ctrl.Result{}, r.updateClusterStatus(ctx, cluster, sm.CurrentState(), err.Error())
			}
		}

		// Update status with new state
		if err := r.updateClusterStatus(ctx, cluster, sm.CurrentState(), ""); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Even if no event, update component counts periodically
		r.updateComponentStatus(ctx, cluster)
		if err := r.Status().Update(ctx, cluster); err != nil {
			log.Error(err, "Failed to update cluster status")
		}
	}

	// Determine requeue time based on state
	requeueAfter := r.getRequeueDuration(sm.CurrentState())
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// initializeStateMachine creates and initializes a state machine based on cluster status
func (r *StormClusterReconciler) initializeStateMachine(cluster *stormv1beta1.StormCluster) *state.StateMachine {
	// Map cluster status phase to state machine state
	var currentState state.State
	log := ctrl.Log.WithName("statemachine")

	// Always create a base state machine first
	sm := state.NewClusterStateMachine()

	// If we have a status phase, set the state machine to that state
	if cluster.Status.Phase != "" {
		switch cluster.Status.Phase {
		case "Pending":
			currentState = state.State(state.ClusterStatePending)
		case "Creating":
			currentState = state.State(state.ClusterStateCreating)
		case "Running":
			currentState = state.State(state.ClusterStateRunning)
		case "Failed":
			currentState = state.State(state.ClusterStateFailed)
		case "Updating":
			currentState = state.State(state.ClusterStateUpdating)
		case "Terminating":
			currentState = state.State(state.ClusterStateTerminating)
		default:
			currentState = state.State(state.ClusterStateUnknown)
		}

		// Force set the current state if we have a known phase
		if currentState != state.State(state.ClusterStateUnknown) {
			sm = state.NewStateMachine(currentState)
			r.setupClusterTransitions(sm)
		}
		log.Info("Initialized state machine from status", "statusPhase", cluster.Status.Phase, "currentState", currentState)
	} else {
		log.Info("No status phase found, using default Unknown state")
	}

	// Set up handlers
	r.setupClusterHandlers(sm)

	return sm
}

// setupClusterTransitions sets up all state transitions
func (r *StormClusterReconciler) setupClusterTransitions(sm *state.StateMachine) {
	// Add all cluster state transitions to the existing state machine
	// From Unknown
	sm.AddTransition(state.State(state.ClusterStateUnknown), state.Event(state.EventCreate), state.State(state.ClusterStatePending))

	// From Pending
	sm.AddTransition(state.State(state.ClusterStatePending), state.Event(state.EventCreate), state.State(state.ClusterStateCreating))

	// From Creating
	sm.AddTransition(state.State(state.ClusterStateCreating), state.Event(state.EventCreateComplete), state.State(state.ClusterStateRunning))
	sm.AddTransition(state.State(state.ClusterStateCreating), state.Event(state.EventCreateFailed), state.State(state.ClusterStateFailed))

	// From Running
	sm.AddTransition(state.State(state.ClusterStateRunning), state.Event(state.EventUnhealthy), state.State(state.ClusterStateFailed))
	sm.AddTransition(state.State(state.ClusterStateRunning), state.Event(state.EventUpdate), state.State(state.ClusterStateUpdating))
	sm.AddTransition(state.State(state.ClusterStateRunning), state.Event(state.EventTerminate), state.State(state.ClusterStateTerminating))

	// From Updating
	sm.AddTransition(state.State(state.ClusterStateUpdating), state.Event(state.EventUpdateComplete), state.State(state.ClusterStateRunning))
	sm.AddTransition(state.State(state.ClusterStateUpdating), state.Event(state.EventUpdateFailed), state.State(state.ClusterStateFailed))

	// From Failed
	sm.AddTransition(state.State(state.ClusterStateFailed), state.Event(state.EventRecover), state.State(state.ClusterStatePending))
	sm.AddTransition(state.State(state.ClusterStateFailed), state.Event(state.EventTerminate), state.State(state.ClusterStateTerminating))
}

// setupClusterHandlers sets up state handlers
func (r *StormClusterReconciler) setupClusterHandlers(sm *state.StateMachine) {
	sm.SetTransitionFunc(func(ctx context.Context, from, to state.State, event state.Event) error {
		log := log.FromContext(ctx)
		log.Info("Cluster state transition", "from", from, "to", to, "event", event)
		return nil
	})
}

// determineNextEvent determines the next event based on current state
func (r *StormClusterReconciler) determineNextEvent(ctx context.Context, clusterCtx *ClusterContext) (state.ClusterEvent, error) {
	log := log.FromContext(ctx)
	cluster := clusterCtx.Cluster
	currentState := clusterCtx.StateMachine.CurrentState()

	log.Info("Determining next event", "currentState", currentState)

	switch state.ClusterState(currentState) {
	case state.ClusterStateUnknown:
		return state.EventCreate, nil

	case state.ClusterStatePending:
		// Check if dependencies are ready
		return state.EventCreate, nil

	case state.ClusterStateCreating:
		// Check if all resources are created and healthy
		if err := r.checkCreationComplete(ctx, cluster); err != nil {
			log.Info("Creation not complete", "error", err.Error())
			return "", nil // Still creating
		}
		return state.EventCreateComplete, nil

	case state.ClusterStateRunning:
		// Check health
		healthy, err := r.isClusterHealthy(ctx, cluster)
		if err != nil {
			return state.EventUnhealthy, nil
		}
		if !healthy {
			return state.EventUnhealthy, nil
		}

		// Perform system-wide health monitoring and recovery
		if r.Coordinator != nil {
			// Only perform health check if cluster is running and has been stable for at least 30 seconds
			if cluster.Status.LastUpdateTime != nil && time.Since(cluster.Status.LastUpdateTime.Time) > 30*time.Second {
				_, _, err := r.Coordinator.PerformSystemHealthCheckAndRecovery(ctx, cluster.Namespace)
				if err != nil {
					// Log error but don't fail reconciliation
					log.Error(err, "Failed to perform system health check and recovery",
						"cluster", cluster.Name,
						"namespace", cluster.Namespace)
				}
			}
		}

		// Check for updates
		if r.needsUpdate(cluster) {
			return state.EventUpdate, nil
		}

		return "", nil // Stay in running

	case state.ClusterStateUpdating:
		// Check if update complete
		if err := r.checkUpdateComplete(ctx, cluster); err != nil {
			return "", nil // Still updating
		}
		return state.EventUpdateComplete, nil

	case state.ClusterStateFailed:
		// Check if recovery is possible
		if r.canRecover(cluster) {
			return state.EventRecover, nil
		}
		return "", nil // Stay in failed

	default:
		return "", nil
	}
}

// executeStateAction executes actions for the current state
func (r *StormClusterReconciler) executeStateAction(ctx context.Context, clusterCtx *ClusterContext) error {
	currentState := clusterCtx.StateMachine.CurrentState()
	cluster := clusterCtx.Cluster

	switch state.ClusterState(currentState) {
	case state.ClusterStateCreating:
		return r.createCluster(ctx, cluster)

	case state.ClusterStateUpdating:
		return r.updateCluster(ctx, cluster)

	case state.ClusterStateTerminating:
		return r.terminateCluster(ctx, cluster)

	default:
		// No action needed for other states
		return nil
	}
}

// createCluster creates cluster resources
func (r *StormClusterReconciler) createCluster(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)
	log.Info("Creating cluster", "cluster", cluster.Name)

	// Create ConfigMap
	if err := r.reconcileConfigMap(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	// Create Nimbus
	if err := r.reconcileNimbus(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Nimbus: %w", err)
	}

	// Create Supervisors
	if err := r.reconcileSupervisors(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Supervisors: %w", err)
	}

	// Create UI
	if cluster.Spec.UI.Enabled {
		if err := r.reconcileUI(ctx, cluster); err != nil {
			return fmt.Errorf("failed to reconcile UI: %w", err)
		}
	}

	// Create Services
	if err := r.reconcileNimbusService(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Nimbus service: %w", err)
	}

	if cluster.Spec.UI.Enabled {
		if err := r.reconcileUIService(ctx, cluster); err != nil {
			return fmt.Errorf("failed to reconcile UI service: %w", err)
		}
	}

	return nil
}

// Additional helper methods...

// checkCreationComplete checks if creation is complete
func (r *StormClusterReconciler) checkCreationComplete(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Determine resource names based on management mode
	configMapName := getConfigMapName(cluster)
	nimbusStatefulSetName := cluster.Name + "-nimbus"
	supervisorDeploymentName := cluster.Name + "-supervisor"
	nimbusServiceName := cluster.Name + "-nimbus"
	uiDeploymentName := cluster.Name + "-ui"
	uiServiceName := cluster.Name + "-ui"

	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil {
		if cluster.Spec.ResourceNames.NimbusStatefulSet != "" {
			nimbusStatefulSetName = cluster.Spec.ResourceNames.NimbusStatefulSet
		}
		if cluster.Spec.ResourceNames.SupervisorDeployment != "" {
			supervisorDeploymentName = cluster.Spec.ResourceNames.SupervisorDeployment
		}
		if cluster.Spec.ResourceNames.NimbusService != "" {
			nimbusServiceName = cluster.Spec.ResourceNames.NimbusService
		}
		if cluster.Spec.ResourceNames.UIDeployment != "" {
			uiDeploymentName = cluster.Spec.ResourceNames.UIDeployment
		}
		if cluster.Spec.ResourceNames.UIService != "" {
			uiServiceName = cluster.Spec.ResourceNames.UIService
		}
	}

	// Check if all expected resources exist
	resources := []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: cluster.Namespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: nimbusStatefulSetName, Namespace: cluster.Namespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: supervisorDeploymentName, Namespace: cluster.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: nimbusServiceName, Namespace: cluster.Namespace}},
	}

	if cluster.Spec.UI.Enabled {
		resources = append(resources,
			&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: uiDeploymentName, Namespace: cluster.Namespace}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: uiServiceName, Namespace: cluster.Namespace}},
		)
	}

	for _, resource := range resources {
		if err := r.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("resource %s not found", resource.GetName())
			}
			return err
		}
	}

	return nil
}

// isClusterHealthy checks if the cluster is healthy
func (r *StormClusterReconciler) isClusterHealthy(ctx context.Context, cluster *stormv1beta1.StormCluster) (bool, error) {
	// Determine resource names based on management mode
	nimbusStatefulSetName := cluster.Name + "-nimbus"
	supervisorDeploymentName := cluster.Name + "-supervisor"

	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil {
		if cluster.Spec.ResourceNames.NimbusStatefulSet != "" {
			nimbusStatefulSetName = cluster.Spec.ResourceNames.NimbusStatefulSet
		}
		if cluster.Spec.ResourceNames.SupervisorDeployment != "" {
			supervisorDeploymentName = cluster.Spec.ResourceNames.SupervisorDeployment
		}
	}

	// Check Nimbus health
	nimbusReady, err := r.getReadyReplicas(ctx, nimbusStatefulSetName, cluster.Namespace, "nimbus")
	if err != nil {
		return false, err
	}

	var nimbusReplicas int32 = 1
	if cluster.Spec.Nimbus != nil && cluster.Spec.Nimbus.Replicas != nil {
		nimbusReplicas = *cluster.Spec.Nimbus.Replicas
	}
	if nimbusReady < nimbusReplicas {
		return false, nil
	}

	// Check Supervisor health
	supervisorReady, err := r.getReadyReplicas(ctx, supervisorDeploymentName, cluster.Namespace, "supervisor")
	if err != nil {
		return false, err
	}

	var supervisorReplicas int32 = 1
	if cluster.Spec.Supervisor != nil && cluster.Spec.Supervisor.Replicas != nil {
		supervisorReplicas = *cluster.Spec.Supervisor.Replicas
	}
	if supervisorReady < supervisorReplicas/2 { // At least half should be ready
		return false, nil
	}

	return true, nil
}

// needsUpdate checks if cluster needs update
func (r *StormClusterReconciler) needsUpdate(cluster *stormv1beta1.StormCluster) bool {
	// Check if image version differs or configuration changes
	// In a real implementation, this would check deployed image versions and config
	return false
}

// canRecover checks if cluster can recover from failed state
func (r *StormClusterReconciler) canRecover(cluster *stormv1beta1.StormCluster) bool {
	// Check failure conditions and determine if recovery is possible
	// For now, always allow recovery attempts
	return true
}

// updateCluster handles cluster update
func (r *StormClusterReconciler) updateCluster(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Re-run creation logic to update resources
	return r.createCluster(ctx, cluster)
}

// terminateCluster handles cluster termination
func (r *StormClusterReconciler) terminateCluster(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Resources will be garbage collected due to owner references
	return nil
}

// checkUpdateComplete checks if update is complete
func (r *StormClusterReconciler) checkUpdateComplete(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Check if all components are updated
	return r.checkCreationComplete(ctx, cluster)
}

// updateClusterStatus updates the cluster status based on state
func (r *StormClusterReconciler) updateClusterStatus(ctx context.Context, cluster *stormv1beta1.StormCluster, currentState state.State, errorMsg string) error {
	cluster.Status.Phase = string(currentState)
	cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Update ready condition
	if errorMsg != "" {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             string(currentState),
			Message:            errorMsg,
		})
	} else {
		readyStates := map[state.State]bool{
			state.State(state.ClusterStateRunning): true,
		}

		conditionStatus := metav1.ConditionFalse
		if readyStates[currentState] {
			conditionStatus = metav1.ConditionTrue
		}

		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             conditionStatus,
			ObservedGeneration: cluster.Generation,
			Reason:             string(currentState),
			Message:            fmt.Sprintf("Cluster is in %s state", currentState),
		})
	}

	// Update component counts
	r.updateComponentStatus(ctx, cluster)

	return r.Status().Update(ctx, cluster)
}

// updateComponentStatus updates component status counts
func (r *StormClusterReconciler) updateComponentStatus(ctx context.Context, cluster *stormv1beta1.StormCluster) {
	log := log.FromContext(ctx)
	// Determine resource names based on management mode
	nimbusStatefulSetName := cluster.Name + "-nimbus"
	supervisorDeploymentName := cluster.Name + "-supervisor"
	uiDeploymentName := cluster.Name + "-ui"

	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil {
		if cluster.Spec.ResourceNames.NimbusStatefulSet != "" {
			nimbusStatefulSetName = cluster.Spec.ResourceNames.NimbusStatefulSet
		}
		if cluster.Spec.ResourceNames.SupervisorDeployment != "" {
			supervisorDeploymentName = cluster.Spec.ResourceNames.SupervisorDeployment
		}
		if cluster.Spec.ResourceNames.UIDeployment != "" {
			uiDeploymentName = cluster.Spec.ResourceNames.UIDeployment
		}
	}

	// Get Nimbus ready count
	nimbusReady, _ := r.getReadyReplicas(ctx, nimbusStatefulSetName, cluster.Namespace, "nimbus")
	cluster.Status.NimbusReady = nimbusReady

	// Update Storm client configuration when Nimbus is ready
	if nimbusReady > 0 && r.ClientManager != nil {
		// Determine UI host based on management mode
		uiHost := fmt.Sprintf("%s-ui", cluster.Name)
		if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.UIService != "" {
			uiHost = cluster.Spec.ResourceNames.UIService
		}

		clientConfig := &storm.ClientConfig{
			Type:       storm.ClientTypeREST, // Use REST for now
			NimbusHost: fmt.Sprintf("%s-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
			NimbusPort: int(cluster.Spec.Nimbus.Thrift.Port),
			UIHost:     uiHost,
			UIPort:     int(cluster.Spec.UI.Service.Port),
		}

		if err := r.ClientManager.UpdateClient(clientConfig); err != nil {
			log.Error(err, "Failed to update Storm client")
			// Continue without failing - client will be updated on next reconcile
		}
	}

	// Get Supervisor ready count
	supervisorReady, err := r.getReadyReplicas(ctx, supervisorDeploymentName, cluster.Namespace, "supervisor")
	if err != nil {
		log.Error(err, "Failed to get supervisor ready replicas", "name", supervisorDeploymentName)
	} else {
		log.Info("Got supervisor ready replicas", "name", supervisorDeploymentName, "ready", supervisorReady)
	}
	cluster.Status.SupervisorReady = supervisorReady

	// Get UI ready count
	if cluster.Spec.UI.Enabled {
		uiReady, _ := r.getReadyReplicas(ctx, uiDeploymentName, cluster.Namespace, "ui")
		cluster.Status.UIReady = uiReady
	}

	// Calculate total slots
	cluster.Status.TotalSlots = cluster.Status.SupervisorReady * cluster.Spec.Supervisor.SlotsPerSupervisor

	// Get actual used slots from Storm API
	if r.ClientManager != nil {
		stormClient, err := r.ClientManager.GetClient()
		if err != nil {
			log.Info("Storm client not available", "error", err.Error())
			cluster.Status.UsedSlots = 0
		} else if stormClient != nil {
			log.Info("Calling GetClusterInfo from Storm API")
			clusterInfo, err := stormClient.GetClusterInfo(ctx)
			if err != nil {
				log.Error(err, "Failed to get cluster info from Storm API")
				cluster.Status.UsedSlots = 0
			} else {
				log.Info("Successfully got cluster info from Storm API",
					"usedSlots", clusterInfo.UsedSlots,
					"totalSlots", clusterInfo.TotalSlots,
					"topologies", clusterInfo.Topologies)
				cluster.Status.UsedSlots = int32(clusterInfo.UsedSlots)
				// Also get topology count
				cluster.Status.TopologyCount = int32(clusterInfo.Topologies)
			}
		}
	} else {
		log.Info("ClientManager is nil")
		cluster.Status.UsedSlots = 0
	}

	cluster.Status.FreeSlots = cluster.Status.TotalSlots - cluster.Status.UsedSlots

	// Format slots info for display
	cluster.Status.SlotsInfo = fmt.Sprintf("%d/%d", cluster.Status.UsedSlots, cluster.Status.TotalSlots)

	// Update endpoints
	// Determine UI service name based on management mode
	uiServiceName := fmt.Sprintf("%s-ui", cluster.Name)
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.UIService != "" {
		uiServiceName = cluster.Spec.ResourceNames.UIService
	}

	cluster.Status.Endpoints.Nimbus = fmt.Sprintf("%s-nimbus.%s.svc.cluster.local:%d",
		cluster.Name, cluster.Namespace, cluster.Spec.Nimbus.Thrift.Port)
	cluster.Status.Endpoints.UI = fmt.Sprintf("%s.%s.svc.cluster.local:%d",
		uiServiceName, cluster.Namespace, cluster.Spec.UI.Service.Port)
	cluster.Status.Endpoints.RestAPI = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/api/v1",
		uiServiceName, cluster.Namespace, cluster.Spec.UI.Service.Port)
}

// getRequeueDuration returns the requeue duration based on state
func (r *StormClusterReconciler) getRequeueDuration(currentState state.State) time.Duration {
	switch state.ClusterState(currentState) {
	case state.ClusterStateRunning:
		return 60 * time.Second // Check every minute when running
	case state.ClusterStateFailed:
		return 5 * time.Minute // Check less frequently when failed
	case state.ClusterStateTerminating:
		return 0 // Don't requeue terminal states
	case state.ClusterStateCreating:
		return 5 * time.Second // Check frequently during creation
	default:
		return 10 * time.Second // Default requeue
	}
}

// handleDeletion handles cluster deletion
func (r *StormClusterReconciler) handleDeletion(ctx context.Context, cluster *stormv1beta1.StormCluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		log.Info("Handling cluster deletion", "cluster", cluster.Name)

		// Remove Storm client connection
		r.ClientManager.RemoveClient()

		// Remove finalizer
		controllerutil.RemoveFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *StormClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// getReadyReplicas returns the number of ready replicas for a workload
func (r *StormClusterReconciler) getReadyReplicas(ctx context.Context, name, namespace, component string) (int32, error) {
	switch component {
	case "nimbus":
		statefulSet := &appsv1.StatefulSet{}
		if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, statefulSet); err != nil {
			return 0, err
		}
		return statefulSet.Status.ReadyReplicas, nil
	case "supervisor", "ui":
		deployment := &appsv1.Deployment{}
		if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployment); err != nil {
			return 0, err
		}
		return deployment.Status.ReadyReplicas, nil
	default:
		return 0, fmt.Errorf("unknown component type: %s", component)
	}
}

// reconcileConfigMap creates or updates the Storm configuration ConfigMap
func (r *StormClusterReconciler) reconcileConfigMap(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	// Determine the ConfigMap name based on management mode
	configMapName := getConfigMapName(cluster)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, configMap, r.Scheme); err != nil {
			return err
		}

		// In reference mode, don't modify existing ConfigMap
		if cluster.Spec.ManagementMode == "reference" && !configMap.CreationTimestamp.IsZero() {
			return nil
		}

		// Build Storm configuration
		stormConfig := r.buildStormConfig(cluster)

		// Log4j2 configuration for console output
		log4j2Config := `<?xml version="1.0" encoding="UTF-8"?>
<configuration monitorInterval="60" shutdownHook="disable">
    <properties>
        <property name="pattern">%d{yyyy-MM-dd HH:mm:ss.SSS} %c{1.} %t [%p] %msg%n</property>
    </properties>
    <appenders>
        <Console name="Console" target="SYSTEM_OUT">
            <PatternLayout charset="UTF-8">
                <pattern>${pattern}</pattern>
            </PatternLayout>
        </Console>
    </appenders>
    <loggers>
        <root level="info">
            <appender-ref ref="Console"/>
        </root>
    </loggers>
</configuration>`

		configMap.Data = map[string]string{
			"storm.yaml":         stormConfig,
			"log4j2-console.xml": log4j2Config,
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled ConfigMap", "name", configMap.Name)
	return nil
}

// buildStormConfig builds the storm.yaml configuration
func (r *StormClusterReconciler) buildStormConfig(cluster *stormv1beta1.StormCluster) string {
	// Get operator defaults
	var operatorDefaults map[string]interface{}
	if r.OperatorConfig != nil {
		operatorDefaults = r.OperatorConfig.Defaults.Storm.Config
	} else {
		operatorDefaults = make(map[string]interface{})
	}

	// Log the defaults we're using
	log := ctrl.Log.WithName("stormcluster")
	log.Info("Using operator defaults", "defaults", operatorDefaults)

	// Get Zookeeper config
	zkConfig := config.GetZookeeperConfig(cluster)

	// Add ZK config to CRD config
	crdConfig := make(map[string]interface{})
	if cluster.Spec.Config != nil {
		for k, v := range cluster.Spec.Config {
			crdConfig[k] = v
		}
	}

	// Add ZK config
	for k, v := range zkConfig {
		crdConfig[k] = v
	}

	// Merge all configs
	clusterName := cluster.Name
	if cluster.Spec.ClusterName != "" {
		clusterName = cluster.Spec.ClusterName
	}
	mergedConfig := config.MergeStormConfig(operatorDefaults, crdConfig, clusterName)

	// Log the merged config
	log.Info("Merged config", "config", mergedConfig)

	// Add standard cluster configs
	mergedConfig["nimbus.seeds"] = []string{fmt.Sprintf("%s-nimbus", cluster.Name)}

	// Add supervisor slots ports
	var slotsPorts []int
	if cluster.Spec.Supervisor != nil {
		for i := 0; i < int(cluster.Spec.Supervisor.SlotsPerSupervisor); i++ {
			slotsPorts = append(slotsPorts, 6700+i)
		}
	}
	mergedConfig["supervisor.slots.ports"] = slotsPorts

	// Add UI port if enabled
	if cluster.Spec.UI != nil && cluster.Spec.UI.Enabled {
		mergedConfig["ui.port"] = 8080
	}

	// Process the config to convert string values to appropriate types
	processedConfig := make(map[string]interface{})
	for key, value := range mergedConfig {
		processedConfig[key] = processConfigValue(key, value)
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(processedConfig)
	if err != nil {
		log.Error(err, "Failed to marshal storm config to YAML")
		// Fall back to a minimal config
		return "# Storm configuration\n# Error generating config: " + err.Error()
	}

	// The yaml library quotes all strings, even numeric ones
	// We need to fix specific fields that Storm expects as unquoted numbers
	configStr := string(yamlBytes)

	// Replace quoted numeric values for known fields
	numericReplacements := map[string]bool{
		"topology.message.timeout.secs":  true,
		"supervisor.worker.timeout.secs": true,
	}

	for field := range numericReplacements {
		// Look for patterns like 'field: "30"' and replace with 'field: 30'
		// The (?m) flag makes ^ and $ match line boundaries
		pattern := fmt.Sprintf(`(?m)^(\s*%s:\s*)"(\d+)"`, regexp.QuoteMeta(field))
		replacement := `${1}${2}`
		configStr = regexp.MustCompile(pattern).ReplaceAllString(configStr, replacement)
	}

	return configStr
}

// processConfigValue converts string values to appropriate types for known numeric fields
func processConfigValue(key string, value interface{}) interface{} {
	// List of known numeric fields in Storm configuration
	numericFields := map[string]bool{
		"topology.message.timeout.secs":      true,
		"supervisor.worker.timeout.secs":     true,
		"nimbus.thrift.port":                 true,
		"ui.port":                            true,
		"storm.zookeeper.connection.timeout": true,
		"storm.zookeeper.session.timeout":    true,
		"topology.debug":                     false, // This is a boolean
	}

	// If it's already the correct type, return as-is
	switch v := value.(type) {
	case string:
		// Check if this field should be numeric
		if isNumeric, ok := numericFields[key]; ok && isNumeric {
			// Try to parse as integer first
			if intVal, err := strconv.Atoi(v); err == nil {
				return intVal
			}
			// Try to parse as float
			if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
				return floatVal
			}
		}
		// Check if it's a boolean field
		if key == "topology.debug" {
			if boolVal, err := strconv.ParseBool(v); err == nil {
				return boolVal
			}
		}
		// Return as string
		return v
	default:
		// Return other types as-is
		return value
	}
}
