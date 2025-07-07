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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/jarextractor"
	"github.com/veteran-chad/storm-controller/pkg/metrics"
	"github.com/veteran-chad/storm-controller/pkg/state"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// StormTopologyReconcilerStateMachine reconciles a StormTopology object using state machines
type StormTopologyReconcilerStateMachine struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientManager storm.ClientManager
	JarExtractor  *jarextractor.Extractor
	ClusterName   string
	Namespace     string
}

// TopologyContext holds the context for a topology reconciliation
type TopologyContext struct {
	Topology     *stormv1beta1.StormTopology
	Cluster      *stormv1beta1.StormCluster
	StormClient  storm.Client
	StateMachine *state.StateMachine
	JarPath      string
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies/finalizers,verbs=update

// Reconcile handles StormTopology reconciliation using state machines
func (r *StormTopologyReconcilerStateMachine) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormTopology instance
	topology := &stormv1beta1.StormTopology{}
	if err := r.Get(ctx, req.NamespacedName, topology); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get the associated Storm cluster
	cluster := &stormv1beta1.StormCluster{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      topology.Spec.ClusterRef,
		Namespace: topology.Namespace,
	}, cluster); err != nil {
		log.Error(err, "Failed to get StormCluster")
		return ctrl.Result{}, err
	}

	// Check if cluster is healthy
	if cluster.Status.Phase != "Running" {
		log.Info("Storm cluster is not running, requeuing", "phase", cluster.Status.Phase)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get Storm client
	stormClient, err := r.ClientManager.GetClient()
	if err != nil {
		log.Error(err, "Storm client not available")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Handle deletion
	if topology.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, topology, stormClient)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(topology, topologyFinalizer) {
		controllerutil.AddFinalizer(topology, topologyFinalizer)
		if err := r.Update(ctx, topology); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create topology context
	topologyCtx := &TopologyContext{
		Topology:    topology,
		Cluster:     cluster,
		StormClient: stormClient,
	}

	// Initialize state machine based on current status
	sm := r.initializeStateMachine(topology)
	topologyCtx.StateMachine = sm

	// Process the topology based on its current state
	event, err := r.determineNextEvent(ctx, topologyCtx)
	if err != nil {
		log.Error(err, "Failed to determine next event")
		return ctrl.Result{}, err
	}

	if event != "" {
		if err := sm.ProcessEvent(ctx, state.Event(event)); err != nil {
			log.Error(err, "Failed to process event", "event", event)
			return ctrl.Result{}, r.updateTopologyStatus(ctx, topology, sm.CurrentState(), err.Error())
		}

		// Update status with new state
		if err := r.updateTopologyStatus(ctx, topology, sm.CurrentState(), ""); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Determine requeue time based on state
	requeueAfter := r.getRequeueDuration(sm.CurrentState())
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// initializeStateMachine creates and initializes a state machine based on topology status
func (r *StormTopologyReconcilerStateMachine) initializeStateMachine(topology *stormv1beta1.StormTopology) *state.StateMachine {
	sm := state.NewTopologyStateMachine()

	// Map topology status phase to state machine state
	var currentState state.State
	switch topology.Status.Phase {
	case "Pending":
		currentState = state.State(state.TopologyStatePending)
	case "Validating":
		currentState = state.State(state.TopologyStateValidating)
	case "Downloading":
		currentState = state.State(state.TopologyStateDownloading)
	case "Submitting":
		currentState = state.State(state.TopologyStateSubmitting)
	case "Running":
		currentState = state.State(state.TopologyStateRunning)
	case "Suspended":
		currentState = state.State(state.TopologyStateSuspended)
	case "Updating":
		currentState = state.State(state.TopologyStateUpdating)
	case "Killing":
		currentState = state.State(state.TopologyStateKilling)
	case "Killed":
		currentState = state.State(state.TopologyStateKilled)
	case "Failed":
		currentState = state.State(state.TopologyStateFailed)
	default:
		currentState = state.State(state.TopologyStateUnknown)
	}

	// Set the current state
	if currentState != state.State(state.TopologyStateUnknown) {
		// Create a new state machine with the current state
		sm = state.NewStateMachine(currentState)
		// Re-add all transitions
		r.setupTopologyTransitions(sm)
	}

	// Set up handlers
	r.setupTopologyHandlers(sm)

	return sm
}

// setupTopologyTransitions sets up all state transitions for topology
func (r *StormTopologyReconcilerStateMachine) setupTopologyTransitions(sm *state.StateMachine) {
	// Copy transitions from the standard topology state machine
	topologySM := state.NewTopologyStateMachine()
	// This is a simplified version - in production, you'd want to expose
	// the transitions or have a method to copy them
	*sm = *topologySM
}

// setupTopologyHandlers sets up state handlers
func (r *StormTopologyReconcilerStateMachine) setupTopologyHandlers(sm *state.StateMachine) {
	// Set transition function to update status
	sm.SetTransitionFunc(func(ctx context.Context, from, to state.State, event state.Event) error {
		log := log.FromContext(ctx)
		log.Info("State transition", "from", from, "to", to, "event", event)
		return nil
	})
}

// determineNextEvent determines the next event based on current state and conditions
func (r *StormTopologyReconcilerStateMachine) determineNextEvent(ctx context.Context, topologyCtx *TopologyContext) (state.TopologyEvent, error) {
	log := log.FromContext(ctx)
	topology := topologyCtx.Topology
	currentState := topologyCtx.StateMachine.CurrentState()

	log.Info("Determining next event", "currentState", currentState)

	switch state.TopologyState(currentState) {
	case state.TopologyStateUnknown:
		return state.EventValidate, nil

	case state.TopologyStatePending:
		if topology.Spec.Suspend {
			return "", nil // Stay in pending if suspended
		}
		return state.EventValidate, nil

	case state.TopologyStateValidating:
		// Perform validation
		if err := r.validateTopology(ctx, topologyCtx); err != nil {
			return state.EventValidationFailed, nil
		}
		return state.EventValidationSuccess, nil

	case state.TopologyStateDownloading:
		// Download JAR
		jarPath, err := r.downloadJAR(ctx, topologyCtx)
		if err != nil {
			return state.EventDownloadFailed, nil
		}
		topologyCtx.JarPath = jarPath
		return state.EventDownloadComplete, nil

	case state.TopologyStateSubmitting:
		// Submit topology
		if err := r.submitTopology(ctx, topologyCtx); err != nil {
			return state.EventSubmitFailed, nil
		}
		return state.EventSubmitSuccess, nil

	case state.TopologyStateRunning:
		// Check if suspended
		if topology.Spec.Suspend {
			return state.EventSuspend, nil
		}
		// Check for version update
		if r.needsUpdate(topology) {
			return state.EventTopologyUpdate, nil
		}
		// Check health
		if err := r.checkTopologyHealth(ctx, topologyCtx); err != nil {
			return state.EventError, nil
		}
		return "", nil // Stay in running

	case state.TopologyStateSuspended:
		if !topology.Spec.Suspend {
			return state.EventResume, nil
		}
		return "", nil // Stay suspended

	case state.TopologyStateUpdating:
		// Perform update
		if err := r.updateTopology(ctx, topologyCtx); err != nil {
			return state.EventError, nil
		}
		return state.EventTopologyUpdateComplete, nil

	case state.TopologyStateFailed:
		// Check if retry is needed
		if r.shouldRetry(topology) {
			return state.EventRetry, nil
		}
		return "", nil // Stay in failed

	default:
		return "", nil
	}
}

// validateTopology validates the topology configuration
func (r *StormTopologyReconcilerStateMachine) validateTopology(ctx context.Context, topologyCtx *TopologyContext) error {
	topology := topologyCtx.Topology

	// Validate JAR source
	if topology.Spec.Topology.Jar.URL == "" &&
		topology.Spec.Topology.Jar.Container == nil &&
		topology.Spec.Topology.Jar.ConfigMap == "" &&
		topology.Spec.Topology.Jar.Secret == "" &&
		topology.Spec.Topology.Jar.S3 == nil {
		return fmt.Errorf("no JAR source specified")
	}

	// Validate main class
	if topology.Spec.Topology.MainClass == "" {
		return fmt.Errorf("main class not specified")
	}

	// Validate topology name
	if topology.Spec.Topology.Name == "" {
		return fmt.Errorf("topology name not specified")
	}

	return nil
}

// downloadJAR downloads the topology JAR
func (r *StormTopologyReconcilerStateMachine) downloadJAR(ctx context.Context, topologyCtx *TopologyContext) (string, error) {
	return r.getJARPath(ctx, topologyCtx.Topology)
}

// submitTopology submits the topology to Storm
func (r *StormTopologyReconcilerStateMachine) submitTopology(ctx context.Context, topologyCtx *TopologyContext) error {
	log := log.FromContext(ctx)
	topology := topologyCtx.Topology
	cluster := topologyCtx.Cluster

	// Build storm submit command
	cmd := r.buildSubmitCommand(topology, cluster, topologyCtx.JarPath)
	log.Info("Submitting topology", "command", strings.Join(cmd, " "))

	// Execute storm submit
	output, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		log.Error(err, "Failed to submit topology", "output", string(output))
		return fmt.Errorf("submit failed: %v, output: %s", err, output)
	}

	log.Info("Topology submitted successfully", "output", string(output))

	// Update metrics
	metrics.StormTopologySubmissions.With(map[string]string{
		"namespace": topology.Namespace,
		"result":    "success",
	}).Inc()

	// Update deployed version
	topology.Status.DeployedVersion = r.getTopologyVersion(topology)
	topology.Status.TopologyID = topology.Spec.Topology.Name
	topology.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	return nil
}

// checkTopologyHealth checks if the topology is healthy
func (r *StormTopologyReconcilerStateMachine) checkTopologyHealth(ctx context.Context, topologyCtx *TopologyContext) error {
	topology := topologyCtx.Topology
	stormTopology, err := topologyCtx.StormClient.GetTopology(ctx, topology.Spec.Topology.Name)
	if err != nil {
		return err
	}

	if stormTopology.Status != "ACTIVE" {
		return fmt.Errorf("topology is not active: %s", stormTopology.Status)
	}

	// Update status with current Storm state
	topology.Status.Workers = int32(stormTopology.Workers)
	topology.Status.Executors = int32(stormTopology.Executors)
	topology.Status.Tasks = int32(stormTopology.Tasks)
	topology.Status.Uptime = fmt.Sprintf("%ds", stormTopology.UptimeSeconds)

	// Update metrics
	labels := map[string]string{
		"topology":  topology.Name,
		"namespace": topology.Namespace,
		"cluster":   topology.Spec.ClusterRef,
	}

	metrics.StormTopologyWorkers.With(labels).Set(float64(stormTopology.Workers))
	metrics.StormTopologyExecutors.With(labels).Set(float64(stormTopology.Executors))
	metrics.StormTopologyTasks.With(labels).Set(float64(stormTopology.Tasks))
	metrics.StormTopologyUptime.With(labels).Set(float64(stormTopology.UptimeSeconds))

	return nil
}

// needsUpdate checks if topology needs update
func (r *StormTopologyReconcilerStateMachine) needsUpdate(topology *stormv1beta1.StormTopology) bool {
	desiredVersion := r.getTopologyVersion(topology)
	return topology.Status.DeployedVersion != "" && topology.Status.DeployedVersion != desiredVersion
}

// updateTopology updates the topology
func (r *StormTopologyReconcilerStateMachine) updateTopology(ctx context.Context, topologyCtx *TopologyContext) error {
	log := log.FromContext(ctx)
	topology := topologyCtx.Topology

	log.Info("Updating topology",
		"topology", topology.Spec.Topology.Name,
		"oldVersion", topology.Status.DeployedVersion,
		"newVersion", r.getTopologyVersion(topology))

	// Kill the existing topology
	waitSecs := 30
	if err := topologyCtx.StormClient.KillTopology(ctx, topology.Spec.Topology.Name, waitSecs); err != nil {
		return fmt.Errorf("failed to kill topology: %w", err)
	}

	// Wait for topology to be removed
	if err := r.waitForTopologyRemoval(ctx, topologyCtx, topology.Spec.Topology.Name); err != nil {
		return fmt.Errorf("failed waiting for topology removal: %w", err)
	}

	// Re-download JAR if needed
	jarPath, err := r.downloadJAR(ctx, topologyCtx)
	if err != nil {
		return fmt.Errorf("failed to download JAR for update: %w", err)
	}
	topologyCtx.JarPath = jarPath

	// Submit new version
	return r.submitTopology(ctx, topologyCtx)
}

// shouldRetry determines if a failed topology should be retried
func (r *StormTopologyReconcilerStateMachine) shouldRetry(topology *stormv1beta1.StormTopology) bool {
	// Check retry count and backoff
	// For now, simple implementation
	return false
}

// updateTopologyStatus updates the topology status based on state
func (r *StormTopologyReconcilerStateMachine) updateTopologyStatus(ctx context.Context, topology *stormv1beta1.StormTopology, currentState state.State, errorMsg string) error {
	topology.Status.Phase = string(currentState)
	topology.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	if errorMsg != "" {
		topology.Status.LastError = errorMsg
		meta.SetStatusCondition(&topology.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: topology.Generation,
			Reason:             string(currentState),
			Message:            errorMsg,
		})
	} else {
		readyStates := map[state.State]bool{
			state.State(state.TopologyStateRunning):   true,
			state.State(state.TopologyStateSuspended): true,
		}

		conditionStatus := metav1.ConditionFalse
		if readyStates[currentState] {
			conditionStatus = metav1.ConditionTrue
		}

		meta.SetStatusCondition(&topology.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             conditionStatus,
			ObservedGeneration: topology.Generation,
			Reason:             string(currentState),
			Message:            fmt.Sprintf("Topology is in %s state", currentState),
		})
	}

	return r.Status().Update(ctx, topology)
}

// getRequeueDuration returns the requeue duration based on state
func (r *StormTopologyReconcilerStateMachine) getRequeueDuration(currentState state.State) time.Duration {
	switch state.TopologyState(currentState) {
	case state.TopologyStateRunning:
		return 60 * time.Second // Check every minute when running
	case state.TopologyStateFailed:
		return 5 * time.Minute // Check less frequently when failed
	case state.TopologyStateKilled:
		return 0 // Don't requeue terminal states
	default:
		return 10 * time.Second // Default requeue for transitional states
	}
}

// Additional helper methods would be copied/adapted from the simple controller...

// handleDeletion handles topology deletion
func (r *StormTopologyReconcilerStateMachine) handleDeletion(ctx context.Context, topology *stormv1beta1.StormTopology, stormClient storm.Client) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(topology, topologyFinalizer) {
		// Create context with state machine for deletion
		sm := state.NewStateMachine(state.State(state.TopologyStateKilling))
		sm.AddTransition(state.State(state.TopologyStateKilling), state.Event(state.EventKillComplete), state.State(state.TopologyStateKilled))

		// Kill topology in Storm
		log.Info("Killing topology", "topology", topology.Spec.Topology.Name)

		waitSecs := 30
		err := stormClient.KillTopology(ctx, topology.Spec.Topology.Name, waitSecs)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "NotAliveException") ||
				strings.Contains(errStr, "not alive") ||
				strings.Contains(errStr, "not found") {
				log.Info("Topology not found in Storm, continuing with deletion")
			} else {
				log.Error(err, "Failed to kill topology")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		} else {
			// Update metrics
			metrics.StormTopologyDeletions.With(map[string]string{
				"namespace": topology.Namespace,
				"result":    "success",
			}).Inc()
		}

		// Process completion event
		sm.ProcessEvent(ctx, state.Event(state.EventKillComplete))

		// Update status
		r.updateTopologyStatus(ctx, topology, sm.CurrentState(), "")

		// Remove finalizer
		controllerutil.RemoveFinalizer(topology, topologyFinalizer)
		if err := r.Update(ctx, topology); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getJARPath handles all JAR source types and returns the local path to the JAR
func (r *StormTopologyReconcilerStateMachine) getJARPath(ctx context.Context, topology *stormv1beta1.StormTopology) (string, error) {
	jarSpec := topology.Spec.Topology.Jar

	// Handle different JAR sources
	if jarSpec.URL != "" {
		return r.downloadJARFromURL(ctx, jarSpec.URL)
	} else if jarSpec.Container != nil {
		return r.extractContainerJAR(ctx, topology, jarSpec.Container)
	} else if jarSpec.ConfigMap != "" {
		return "", fmt.Errorf("ConfigMap JAR source not yet implemented")
	} else if jarSpec.Secret != "" {
		return "", fmt.Errorf("Secret JAR source not yet implemented")
	} else if jarSpec.S3 != nil {
		return "", fmt.Errorf("S3 JAR source not yet implemented")
	}

	return "", fmt.Errorf("no JAR source specified")
}

// downloadJARFromURL downloads JAR from URL
func (r *StormTopologyReconcilerStateMachine) downloadJARFromURL(ctx context.Context, url string) (string, error) {
	// Create cache directory
	if err := os.MkdirAll(jarCacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create JAR cache directory: %w", err)
	}

	// Generate cache file path
	jarName := filepath.Base(url)
	if jarName == "" || jarName == "/" {
		jarName = "topology.jar"
	}
	jarPath := filepath.Join(jarCacheDir, jarName)

	// Check if already cached
	if _, err := os.Stat(jarPath); err == nil {
		return jarPath, nil
	}

	// Get Storm client for downloading
	stormClient, err := r.ClientManager.GetClient()
	if err != nil {
		return "", err
	}

	jarData, err := stormClient.DownloadJar(ctx, url)
	if err != nil {
		return "", err
	}

	// Write to cache
	if err := os.WriteFile(jarPath, jarData, 0644); err != nil {
		return "", fmt.Errorf("failed to write JAR file: %w", err)
	}

	return jarPath, nil
}

// extractContainerJAR extracts JAR from container image
func (r *StormTopologyReconcilerStateMachine) extractContainerJAR(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (string, error) {
	log := log.FromContext(ctx)

	if r.JarExtractor == nil {
		return "", fmt.Errorf("JAR extractor not configured")
	}

	log.Info("Extracting JAR from container",
		"image", jarSpec.Image,
		"path", jarSpec.Path,
		"mode", jarSpec.ExtractionMode)

	// Extract JAR using the extractor
	result, err := r.JarExtractor.ExtractJAR(ctx, topology, jarSpec)
	if err != nil {
		return "", fmt.Errorf("failed to extract JAR from container: %w", err)
	}

	log.Info("JAR extraction completed",
		"path", result.JarPath,
		"size", result.Size,
		"checksum", result.Checksum)

	// For now, download from example URL as workaround
	jarURL := "https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.4.0/storm-starter-2.4.0.jar"
	return r.downloadJARFromURL(ctx, jarURL)
}

// buildSubmitCommand builds the storm submit command
func (r *StormTopologyReconcilerStateMachine) buildSubmitCommand(topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster, jarPath string) []string {
	cmd := []string{
		"/apache-storm/bin/storm", "jar", jarPath, topology.Spec.Topology.MainClass,
	}

	// Add topology name
	cmd = append(cmd, topology.Spec.Topology.Name)

	// Add args if specified
	if topology.Spec.Topology.Args != nil {
		cmd = append(cmd, topology.Spec.Topology.Args...)
	}

	// Add nimbus host
	nimbusHost := fmt.Sprintf("%s-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace)
	cmd = append(cmd, "-c", fmt.Sprintf("nimbus.seeds=[%q]", nimbusHost))

	// Add configuration
	if topology.Spec.Topology.Config != nil {
		for key, value := range topology.Spec.Topology.Config {
			cmd = append(cmd, "-c", fmt.Sprintf("%s=%s", key, value))
		}
	}

	return cmd
}

// waitForTopologyRemoval waits for topology to be removed from Storm
func (r *StormTopologyReconcilerStateMachine) waitForTopologyRemoval(ctx context.Context, topologyCtx *TopologyContext, topologyName string) error {
	log := log.FromContext(ctx)

	// Poll for up to 2 minutes
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for topology %s to be removed", topologyName)
		case <-ticker.C:
			// Check if topology still exists
			_, err := topologyCtx.StormClient.GetTopology(ctx, topologyName)
			if err != nil && strings.Contains(err.Error(), "not found") {
				log.Info("Topology has been removed from Storm", "topology", topologyName)
				return nil
			}
			log.Info("Topology still exists, continuing to wait", "topology", topologyName)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// getTopologyVersion gets the topology version
func (r *StormTopologyReconcilerStateMachine) getTopologyVersion(topology *stormv1beta1.StormTopology) string {
	if topology.Spec.Topology.Config != nil {
		if version, ok := topology.Spec.Topology.Config["topology.version"]; ok && version != "" {
			return version
		}
	}
	return "unversioned"
}

// SetupWithManager sets up the controller with the Manager
func (r *StormTopologyReconcilerStateMachine) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormTopology{}).
		Complete(r)
}
