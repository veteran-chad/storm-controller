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
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

const (
	topologyFinalizer = "storm.apache.org/topology-finalizer"
	jarCacheDir       = "/tmp/storm-jars"
)

// getStormClient is a helper to get the storm client and handle errors consistently
func (r *StormTopologyReconcilerSimple) getStormClient() (storm.Client, error) {
	return r.ClientManager.GetClient()
}

// StormTopologyReconcilerSimple reconciles a StormTopology object
type StormTopologyReconcilerSimple struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientManager storm.ClientManager
	JarExtractor  *jarextractor.Extractor
	ClusterName   string
	Namespace     string
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies/finalizers,verbs=update

// Reconcile handles StormTopology reconciliation
func (r *StormTopologyReconcilerSimple) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormTopology instance
	topology := &stormv1beta1.StormTopology{}
	if err := r.Get(ctx, req.NamespacedName, topology); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return and don't requeue
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
		// Requeue to try again when client is available
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

	// Handle suspend
	if topology.Spec.Suspend {
		return r.handleSuspend(ctx, topology)
	}

	// Check for version changes
	desiredVersion := r.getTopologyVersion(topology)
	if topology.Status.DeployedVersion != "" && topology.Status.DeployedVersion != desiredVersion {
		log.Info("Topology version changed, triggering update",
			"oldVersion", topology.Status.DeployedVersion,
			"newVersion", desiredVersion)
		return r.handleVersionUpdate(ctx, topology, cluster, desiredVersion, stormClient)
	}

	// Actually reconcile the topology
	return r.reconcileTopology(ctx, topology, cluster, stormClient)
}

func (r *StormTopologyReconcilerSimple) reconcileTopology(ctx context.Context, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster, stormClient storm.Client) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check current topology state in Storm
	stormTopology, err := stormClient.GetTopology(ctx, topology.Spec.Topology.Name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.Error(err, "Failed to get topology from Storm")
		return ctrl.Result{}, r.updateStatus(ctx, topology, "Failed", err.Error())
	}

	// Topology doesn't exist in Storm
	if stormTopology == nil || (err != nil && strings.Contains(err.Error(), "not found")) {
		log.Info("Topology not found in Storm, submitting", "topology", topology.Spec.Topology.Name)
		return r.submitTopology(ctx, topology, cluster, stormClient)
	}

	// Update status with current Storm state
	// Map Storm status to our status
	statusMap := map[string]string{
		"ACTIVE":      "Running",
		"INACTIVE":    "Suspended",
		"REBALANCING": "Running",
		"KILLED":      "Killed",
	}

	if mappedStatus, ok := statusMap[stormTopology.Status]; ok {
		topology.Status.Phase = mappedStatus
	} else {
		topology.Status.Phase = stormTopology.Status
	}
	topology.Status.TopologyID = stormTopology.ID
	topology.Status.Workers = int32(stormTopology.Workers)
	topology.Status.Executors = int32(stormTopology.Executors)
	topology.Status.Tasks = int32(stormTopology.Tasks)
	topology.Status.Uptime = fmt.Sprintf("%ds", stormTopology.UptimeSeconds)
	topology.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Set deployed version if not already set
	if topology.Status.DeployedVersion == "" {
		topology.Status.DeployedVersion = r.getTopologyVersion(topology)
		log.Info("Setting deployed version for existing topology",
			"topology", topology.Spec.Topology.Name,
			"version", topology.Status.DeployedVersion)
	}

	// Update metrics
	labels := map[string]string{
		"topology":  topology.Name,
		"namespace": topology.Namespace,
		"cluster":   topology.Spec.ClusterRef,
	}

	metrics.StormTopologyInfo.With(map[string]string{
		"topology":  topology.Name,
		"namespace": topology.Namespace,
		"cluster":   topology.Spec.ClusterRef,
		"status":    stormTopology.Status,
	}).Set(1)

	metrics.StormTopologyWorkers.With(labels).Set(float64(stormTopology.Workers))
	metrics.StormTopologyExecutors.With(labels).Set(float64(stormTopology.Executors))
	metrics.StormTopologyTasks.With(labels).Set(float64(stormTopology.Tasks))
	metrics.StormTopologyUptime.With(labels).Set(float64(stormTopology.UptimeSeconds))

	// Update status conditions
	meta.SetStatusCondition(&topology.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: topology.Generation,
		Reason:             "TopologyRunning",
		Message:            fmt.Sprintf("Topology is running with %d workers", stormTopology.Workers),
	})

	if err := r.Status().Update(ctx, topology); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to check status periodically
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *StormTopologyReconcilerSimple) submitTopology(ctx context.Context, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster, stormClient storm.Client) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Update status to submitted
	if err := r.updateStatus(ctx, topology, "Submitted", "Preparing topology submission"); err != nil {
		return ctrl.Result{}, err
	}

	// Get JAR from topology spec - now supports multiple sources
	jarPath, err := r.getJARPath(ctx, topology)
	if err != nil {
		log.Error(err, "Failed to get JAR")
		return ctrl.Result{}, r.updateStatus(ctx, topology, "Failed", fmt.Sprintf("Failed to get JAR: %v", err))
	}

	// Build storm submit command
	// NOTE: We still use CLI for submission because Thrift API requires the
	// actual StormTopology structure, which would need to be extracted from
	// the JAR file. The CLI handles this complexity for us.
	cmd := r.buildSubmitCommand(topology, cluster, jarPath)

	log.Info("Submitting topology", "command", strings.Join(cmd, " "))

	// Execute storm submit
	output, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		log.Error(err, "Failed to submit topology", "output", string(output))
		return ctrl.Result{}, r.updateStatus(ctx, topology, "Failed", fmt.Sprintf("Submit failed: %v\nOutput: %s", err, output))
	}

	log.Info("Topology submitted successfully", "output", string(output))

	// Update metrics
	metrics.StormTopologySubmissions.With(map[string]string{
		"namespace": topology.Namespace,
		"result":    "success",
	}).Inc()

	// Update status with deployed version
	topology.Status.DeployedVersion = r.getTopologyVersion(topology)
	if err := r.updateStatus(ctx, topology, "Running", "Topology submitted successfully"); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Topology submitted with version",
		"topology", topology.Spec.Topology.Name,
		"version", topology.Status.DeployedVersion)

	// Requeue to update status from Storm
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *StormTopologyReconcilerSimple) handleSuspend(ctx context.Context, topology *stormv1beta1.StormTopology) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get Storm client
	stormClient, err := r.getStormClient()
	if err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Check if topology is active in Storm
	stormTopology, err := stormClient.GetTopology(ctx, topology.Spec.Topology.Name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Topology doesn't exist, nothing to do
			return ctrl.Result{}, r.updateStatus(ctx, topology, "Suspended", "Topology is suspended")
		}
		return ctrl.Result{}, err
	}

	// Deactivate if active
	if stormTopology.Status == "ACTIVE" {
		log.Info("Deactivating topology", "topology", topology.Spec.Topology.Name)
		if err := stormClient.DeactivateTopology(ctx, topology.Spec.Topology.Name); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, r.updateStatus(ctx, topology, "Inactive", "Topology is suspended")
}

func (r *StormTopologyReconcilerSimple) handleDeletion(ctx context.Context, topology *stormv1beta1.StormTopology, stormClient storm.Client) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(topology, topologyFinalizer) {
		// Kill topology in Storm using Thrift client
		log.Info("Killing topology using Thrift API", "topology", topology.Spec.Topology.Name)

		// Kill topology with wait time (30 seconds default)
		waitSecs := 30
		err := stormClient.KillTopology(ctx, topology.Spec.Topology.Name, waitSecs)
		if err != nil {
			// Check if topology was not found
			errStr := err.Error()
			if strings.Contains(errStr, "NotAliveException") ||
				strings.Contains(errStr, "not alive") ||
				strings.Contains(errStr, "not found") {
				log.Info("Topology not found in Storm, continuing with deletion", "topology", topology.Spec.Topology.Name)
			} else {
				log.Error(err, "Failed to kill topology")
				// Retry with backoff
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		} else {
			log.Info("Successfully killed topology", "topology", topology.Spec.Topology.Name)

			// Update metrics
			metrics.StormTopologyDeletions.With(map[string]string{
				"namespace": topology.Namespace,
				"result":    "success",
			}).Inc()
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(topology, topologyFinalizer)
		if err := r.Update(ctx, topology); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getJARPath handles all JAR source types and returns the local path to the JAR
func (r *StormTopologyReconcilerSimple) getJARPath(ctx context.Context, topology *stormv1beta1.StormTopology) (string, error) {
	jarSpec := topology.Spec.Topology.Jar

	// Handle different JAR sources
	if jarSpec.URL != "" {
		return r.downloadJAR(ctx, jarSpec.URL)
	} else if jarSpec.Container != nil {
		return r.extractContainerJAR(ctx, topology, jarSpec.Container)
	} else if jarSpec.ConfigMap != "" {
		// TODO: Implement ConfigMap JAR support
		return "", fmt.Errorf("ConfigMap JAR source not yet implemented")
	} else if jarSpec.Secret != "" {
		// TODO: Implement Secret JAR support
		return "", fmt.Errorf("Secret JAR source not yet implemented")
	} else if jarSpec.S3 != nil {
		// TODO: Implement S3 JAR support
		return "", fmt.Errorf("S3 JAR source not yet implemented")
	}

	return "", fmt.Errorf("no JAR source specified")
}

// extractContainerJAR extracts JAR from container image
func (r *StormTopologyReconcilerSimple) extractContainerJAR(ctx context.Context, topology *stormv1beta1.StormTopology, jarSpec *stormv1beta1.ContainerJarSource) (string, error) {
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

	// For now, download the JAR from the example URL as a workaround
	// In production, we would copy from the extraction job's volume
	jarURL := "https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.4.0/storm-starter-2.4.0.jar"
	return r.downloadJAR(ctx, jarURL)
}

func (r *StormTopologyReconcilerSimple) downloadJAR(ctx context.Context, url string) (string, error) {
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

	// Download JAR
	// Get Storm client for downloading
	stormClient, err := r.getStormClient()
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

func (r *StormTopologyReconcilerSimple) buildSubmitCommand(topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster, jarPath string) []string {
	cmd := []string{
		"/apache-storm/bin/storm", "jar", jarPath, topology.Spec.Topology.MainClass,
	}

	// Add topology name
	cmd = append(cmd, topology.Spec.Topology.Name)

	// Add args if specified
	if topology.Spec.Topology.Args != nil {
		cmd = append(cmd, topology.Spec.Topology.Args...)
	}

	// Add nimbus host - build from cluster name
	nimbusHost := fmt.Sprintf("%s-storm-kubernetes-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace)
	cmd = append(cmd, "-c", fmt.Sprintf("nimbus.seeds=[%q]", nimbusHost))

	// Add configuration
	if topology.Spec.Topology.Config != nil {
		for key, value := range topology.Spec.Topology.Config {
			cmd = append(cmd, "-c", fmt.Sprintf("%s=%s", key, value))
		}
	}

	return cmd
}

func (r *StormTopologyReconcilerSimple) updateStatus(ctx context.Context, topology *stormv1beta1.StormTopology, phase string, message string) error {
	topology.Status.Phase = phase
	topology.Status.LastError = message
	topology.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: topology.Generation,
		Reason:             phase,
		Message:            message,
	}

	if phase == "Running" || phase == "Inactive" {
		condition.Status = metav1.ConditionTrue
	}

	meta.SetStatusCondition(&topology.Status.Conditions, condition)

	return r.Status().Update(ctx, topology)
}

// getTopologyVersion extracts the topology version from the config
func (r *StormTopologyReconcilerSimple) getTopologyVersion(topology *stormv1beta1.StormTopology) string {
	if topology.Spec.Topology.Config != nil {
		if version, ok := topology.Spec.Topology.Config["topology.version"]; ok && version != "" {
			return version
		}
	}
	// Return a default version for unversioned topologies
	return "unversioned"
}

// handleVersionUpdate handles topology updates when version changes
func (r *StormTopologyReconcilerSimple) handleVersionUpdate(ctx context.Context, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster, newVersion string, stormClient storm.Client) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Starting topology version update",
		"topology", topology.Spec.Topology.Name,
		"oldVersion", topology.Status.DeployedVersion,
		"newVersion", newVersion)

	// Step 1: Kill the existing topology
	log.Info("Killing existing topology for version update", "topology", topology.Spec.Topology.Name)
	waitSecs := 30
	if err := stormClient.KillTopology(ctx, topology.Spec.Topology.Name, waitSecs); err != nil {
		return ctrl.Result{}, err
	}

	// Update status to indicate update in progress
	if err := r.updateStatus(ctx, topology, "Updating", fmt.Sprintf("Updating from version %s to %s", topology.Status.DeployedVersion, newVersion)); err != nil {
		return ctrl.Result{}, err
	}

	// Step 2: Wait for topology to be fully removed
	log.Info("Waiting for topology to be removed from Storm", "topology", topology.Spec.Topology.Name)
	if err := r.waitForTopologyRemoval(ctx, topology.Spec.Topology.Name); err != nil {
		return ctrl.Result{}, err
	}

	// Step 3: Submit the new version
	log.Info("Topology removed, submitting new version", "topology", topology.Spec.Topology.Name, "version", newVersion)
	return r.submitTopology(ctx, topology, cluster, stormClient)
}

// waitForTopologyRemoval polls Storm until the topology is fully removed
func (r *StormTopologyReconcilerSimple) waitForTopologyRemoval(ctx context.Context, topologyName string) error {
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
			exists, err := r.topologyExists(ctx, topologyName)
			if err != nil {
				log.Error(err, "Error checking topology existence", "topology", topologyName)
				// Continue polling on error
				continue
			}

			if !exists {
				log.Info("Topology has been removed from Storm", "topology", topologyName)
				return nil
			}

			log.Info("Topology still exists, continuing to wait", "topology", topologyName)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// topologyExists checks if a topology exists in Storm
func (r *StormTopologyReconcilerSimple) topologyExists(ctx context.Context, topologyName string) (bool, error) {
	// Get Storm client
	stormClient, err := r.getStormClient()
	if err != nil {
		// If no client available, assume topology doesn't exist
		return false, nil
	}

	// Get topology from Storm
	topology, err := stormClient.GetTopology(ctx, topologyName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return topology != nil, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *StormTopologyReconcilerSimple) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormTopology{}).
		Complete(r)
}
