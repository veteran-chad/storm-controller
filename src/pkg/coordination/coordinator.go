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

package coordination

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// ResourceCoordinator handles coordination between Storm resources
type ResourceCoordinator struct {
	client.Client
	ClientManager  storm.ClientManager
	HealthMonitor  *HealthMonitor
	Provisioner    *WorkerPoolProvisioner
	CrossMonitor   *CrossResourceMonitor
	RecoveryEngine *AutoRecoveryEngine
	Scheme         *runtime.Scheme
}

// NewResourceCoordinator creates a new resource coordinator
func NewResourceCoordinator(client client.Client, clientManager storm.ClientManager, scheme *runtime.Scheme) *ResourceCoordinator {
	healthMonitor := NewHealthMonitor(client, clientManager)
	crossMonitor := NewCrossResourceMonitor(client, healthMonitor)

	coordinator := &ResourceCoordinator{
		Client:        client,
		ClientManager: clientManager,
		HealthMonitor: healthMonitor,
		CrossMonitor:  crossMonitor,
		Scheme:        scheme,
	}

	// Initialize provisioner and recovery engine with circular references
	coordinator.Provisioner = NewWorkerPoolProvisioner(client, coordinator, scheme)
	coordinator.RecoveryEngine = NewAutoRecoveryEngine(client, crossMonitor)

	return coordinator
}

// ClusterReadinessResult represents the readiness status of a cluster
type ClusterReadinessResult struct {
	Ready           bool
	Reason          string
	Message         string
	RecommendedWait time.Duration
}

// TopologyDeploymentResult represents the result of topology deployment coordination
type TopologyDeploymentResult struct {
	CanDeploy       bool
	Reason          string
	Message         string
	RecommendedWait time.Duration
}

// WorkerPoolRequirement represents worker pool requirements for a topology
type WorkerPoolRequirement struct {
	MinWorkers     int32
	RecommendedCPU string
	RecommendedRAM string
	RequiredLabels map[string]string
	Tolerations    []string
}

// CheckClusterReadiness validates if a cluster is ready for topology deployment
func (rc *ResourceCoordinator) CheckClusterReadiness(ctx context.Context, clusterRef, namespace string) (*ClusterReadinessResult, error) {
	log := log.FromContext(ctx)

	// Get the cluster
	cluster := &stormv1beta1.StormCluster{}
	if err := rc.Get(ctx, types.NamespacedName{
		Name:      clusterRef,
		Namespace: namespace,
	}, cluster); err != nil {
		if errors.IsNotFound(err) {
			return &ClusterReadinessResult{
				Ready:           false,
				Reason:          "ClusterNotFound",
				Message:         fmt.Sprintf("StormCluster %s not found", clusterRef),
				RecommendedWait: 60 * time.Second,
			}, nil
		}
		return nil, err
	}

	// Check cluster phase
	if cluster.Status.Phase != "Running" {
		return &ClusterReadinessResult{
			Ready:           false,
			Reason:          "ClusterNotRunning",
			Message:         fmt.Sprintf("Cluster is in %s phase, waiting for Running", cluster.Status.Phase),
			RecommendedWait: 30 * time.Second,
		}, nil
	}

	// Use health monitor for comprehensive readiness check
	ready, message, waitTime := rc.HealthMonitor.IsClusterReadyForTopology(ctx, clusterRef, namespace)

	var reason string
	if ready {
		reason = "ClusterReady"
	} else {
		reason = "ClusterNotReady"
		// Log detailed health information for troubleshooting
		health, err := rc.HealthMonitor.CheckClusterHealth(ctx, clusterRef, namespace)
		if err == nil {
			log.Info("Cluster health details",
				"cluster", clusterRef,
				"overallStatus", health.OverallStatus,
				"readinessScore", health.ReadinessScore,
				"recommendations", strings.Join(health.Recommendations, "; "))
		}
	}

	return &ClusterReadinessResult{
		Ready:           ready,
		Reason:          reason,
		Message:         message,
		RecommendedWait: waitTime,
	}, nil
}

// ValidateTopologyDeployment checks if a topology can be deployed considering cluster state and resource requirements
func (rc *ResourceCoordinator) ValidateTopologyDeployment(ctx context.Context, topology *stormv1beta1.StormTopology) (*TopologyDeploymentResult, error) {
	// First check cluster readiness
	clusterReadiness, err := rc.CheckClusterReadiness(ctx, topology.Spec.ClusterRef, topology.Namespace)
	if err != nil {
		return nil, err
	}

	if !clusterReadiness.Ready {
		return &TopologyDeploymentResult{
			CanDeploy:       false,
			Reason:          clusterReadiness.Reason,
			Message:         clusterReadiness.Message,
			RecommendedWait: clusterReadiness.RecommendedWait,
		}, nil
	}

	// Check for resource conflicts with existing topologies
	conflictResult, err := rc.checkResourceConflicts(ctx, topology)
	if err != nil {
		return nil, err
	}

	if !conflictResult.CanDeploy {
		return conflictResult, nil
	}

	// TODO: Check worker pool availability when WorkerPoolRef field is added to topology spec
	// For now, worker pools are managed separately and linked via TopologyRef

	return &TopologyDeploymentResult{
		CanDeploy: true,
		Reason:    "ValidationPassed",
		Message:   "Topology can be deployed",
	}, nil
}

// checkResourceConflicts checks for resource conflicts with existing topologies
func (rc *ResourceCoordinator) checkResourceConflicts(ctx context.Context, topology *stormv1beta1.StormTopology) (*TopologyDeploymentResult, error) {
	// Get all topologies in the same namespace
	topologyList := &stormv1beta1.StormTopologyList{}
	if err := rc.List(ctx, topologyList, client.InNamespace(topology.Namespace)); err != nil {
		return nil, err
	}

	// Check for name conflicts
	for _, existingTopology := range topologyList.Items {
		if existingTopology.Name == topology.Name && existingTopology.UID != topology.UID {
			// This is a different topology with the same name
			if existingTopology.Status.Phase == "Running" || existingTopology.Status.Phase == "Submitting" {
				return &TopologyDeploymentResult{
					CanDeploy:       false,
					Reason:          "NameConflict",
					Message:         fmt.Sprintf("Another topology with name %s is already running", topology.Name),
					RecommendedWait: 60 * time.Second,
				}, nil
			}
		}

		// TODO: Check for worker pool contention when WorkerPoolRef is added to topology spec
		// For now, resource contention is handled by Storm itself
	}

	return &TopologyDeploymentResult{
		CanDeploy: true,
		Reason:    "NoConflicts",
		Message:   "No resource conflicts detected",
	}, nil
}

// validateWorkerPoolAvailability checks if the specified worker pool is available
// TODO: This function will be implemented when WorkerPoolRef is added to topology spec
func (rc *ResourceCoordinator) validateWorkerPoolAvailability(ctx context.Context, topology *stormv1beta1.StormTopology) (*TopologyDeploymentResult, error) {
	return &TopologyDeploymentResult{
		CanDeploy: true,
		Reason:    "WorkerPoolValidationSkipped",
		Message:   "Worker pool validation not implemented yet",
	}, nil
}

// CalculateWorkerPoolRequirements calculates optimal worker pool configuration for a topology
func (rc *ResourceCoordinator) CalculateWorkerPoolRequirements(ctx context.Context, topology *stormv1beta1.StormTopology) (*WorkerPoolRequirement, error) {
	// Default requirements
	requirement := &WorkerPoolRequirement{
		MinWorkers:     1,
		RecommendedCPU: "1000m",
		RecommendedRAM: "2Gi",
		RequiredLabels: make(map[string]string),
		Tolerations:    []string{},
	}

	// Calculate based on topology workers spec
	if topology.Spec.Workers != nil && topology.Spec.Workers.Replicas > 0 {
		requirement.MinWorkers = topology.Spec.Workers.Replicas
	}

	// Use topology resource requirements if specified
	if topology.Spec.Workers != nil && topology.Spec.Workers.Resources.Requests != nil {
		if cpu, exists := topology.Spec.Workers.Resources.Requests["cpu"]; exists {
			requirement.RecommendedCPU = cpu.String()
		}
		if memory, exists := topology.Spec.Workers.Resources.Requests["memory"]; exists {
			requirement.RecommendedRAM = memory.String()
		}
	}

	// Add topology-specific labels
	requirement.RequiredLabels["storm.apache.org/topology"] = topology.Name
	requirement.RequiredLabels["storm.apache.org/cluster"] = topology.Spec.ClusterRef

	return requirement, nil
}

// GetClusterCapacity returns the current capacity and utilization of a cluster
func (rc *ResourceCoordinator) GetClusterCapacity(ctx context.Context, clusterRef, namespace string) (*ClusterCapacity, error) {
	cluster := &stormv1beta1.StormCluster{}
	if err := rc.Get(ctx, types.NamespacedName{
		Name:      clusterRef,
		Namespace: namespace,
	}, cluster); err != nil {
		return nil, err
	}

	// Get all topologies using this cluster
	topologyList := &stormv1beta1.StormTopologyList{}
	if err := rc.List(ctx, topologyList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// Calculate used resources
	var usedWorkers int32
	runningTopologies := 0
	for _, topology := range topologyList.Items {
		if topology.Spec.ClusterRef == clusterRef && topology.Status.Phase == "Running" {
			if topology.Spec.Workers != nil {
				usedWorkers += topology.Spec.Workers.Replicas
			} else {
				usedWorkers += 1 // Default to 1 worker if not specified
			}
			runningTopologies++
		}
	}

	// Calculate total capacity (simplified calculation)
	totalSlots := cluster.Spec.Supervisor.Replicas * cluster.Spec.Supervisor.WorkerSlots

	return &ClusterCapacity{
		TotalSlots:         totalSlots,
		UsedSlots:          usedWorkers,
		AvailableSlots:     totalSlots - usedWorkers,
		RunningTopologies:  int32(runningTopologies),
		UtilizationPercent: float64(usedWorkers) / float64(totalSlots) * 100,
	}, nil
}

// PerformSystemHealthCheckAndRecovery performs comprehensive system health check and automated recovery
func (rc *ResourceCoordinator) PerformSystemHealthCheckAndRecovery(ctx context.Context, namespace string) (*SystemHealthStatus, *RecoveryPlan, error) {
	log := log.FromContext(ctx)

	log.Info("Starting system-wide health check and recovery analysis", "namespace", namespace)

	// Get comprehensive system health status
	systemHealth, err := rc.CrossMonitor.CheckSystemHealth(ctx, namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check system health: %w", err)
	}

	log.Info("System health check completed",
		"overallHealth", systemHealth.OverallHealth,
		"healthScore", systemHealth.HealthScore,
		"criticalIssues", len(systemHealth.CriticalIssues))

	// Perform automated recovery analysis and execution
	recoveryPlan, err := rc.RecoveryEngine.AnalyzeAndRecover(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to perform automated recovery")
		return systemHealth, nil, err
	}

	if recoveryPlan != nil && len(recoveryPlan.RecoveryActions) > 0 {
		log.Info("Recovery plan executed",
			"actions", len(recoveryPlan.RecoveryActions),
			"priority", recoveryPlan.Priority,
			"riskLevel", recoveryPlan.RiskLevel)
	}

	return systemHealth, recoveryPlan, nil
}

// GetSystemHealthSummary returns a summary of system health for monitoring
func (rc *ResourceCoordinator) GetSystemHealthSummary(ctx context.Context, namespace string) (map[string]interface{}, error) {
	systemHealth, _, err := rc.PerformSystemHealthCheckAndRecovery(ctx, namespace)
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"overall_health":     systemHealth.OverallHealth,
		"health_score":       systemHealth.HealthScore,
		"total_clusters":     systemHealth.SystemMetrics.TotalClusters,
		"healthy_clusters":   systemHealth.SystemMetrics.HealthyClusters,
		"total_topologies":   systemHealth.SystemMetrics.TotalTopologies,
		"running_topologies": systemHealth.SystemMetrics.RunningTopologies,
		"failed_topologies":  systemHealth.SystemMetrics.FailedTopologies,
		"total_worker_pools": systemHealth.SystemMetrics.TotalWorkerPools,
		"ready_worker_pools": systemHealth.SystemMetrics.ReadyWorkerPools,
		"critical_issues":    len(systemHealth.CriticalIssues),
		"recommendations":    systemHealth.Recommendations,
		"last_checked":       systemHealth.LastChecked,
	}

	return summary, nil
}

// ClusterCapacity represents cluster capacity information
type ClusterCapacity struct {
	TotalSlots         int32
	UsedSlots          int32
	AvailableSlots     int32
	RunningTopologies  int32
	UtilizationPercent float64
}
