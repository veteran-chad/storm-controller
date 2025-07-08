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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusDegraded  HealthStatus = "Degraded"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

// ComponentHealth represents the health of a Storm component
type ComponentHealth struct {
	Component    string
	Status       HealthStatus
	Message      string
	LastChecked  time.Time
	CheckDetails map[string]interface{}
}

// ClusterHealth represents the overall health of a Storm cluster
type ClusterHealth struct {
	OverallStatus    HealthStatus
	Components       []ComponentHealth
	TopologyCapacity *TopologyCapacity
	ResourceMetrics  *ResourceMetrics
	Recommendations  []string
	LastHealthCheck  time.Time
	ReadinessScore   int // 0-100, where 100 is fully ready
}

// TopologyCapacity represents topology deployment capacity
type TopologyCapacity struct {
	MaxTopologies      int32
	RunningTopologies  int32
	AvailableSlots     int32
	TotalSlots         int32
	UtilizationPercent float64
}

// ResourceMetrics represents resource usage metrics
type ResourceMetrics struct {
	CPUUtilization    float64
	MemoryUtilization float64
	NetworkLatency    time.Duration
	DiskUtilization   float64
}

// HealthMonitor monitors the health of Storm clusters and components
type HealthMonitor struct {
	client.Client
	ClientManager storm.ClientManager
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(client client.Client, clientManager storm.ClientManager) *HealthMonitor {
	return &HealthMonitor{
		Client:        client,
		ClientManager: clientManager,
	}
}

// CheckClusterHealth performs comprehensive health check of a Storm cluster
func (hm *HealthMonitor) CheckClusterHealth(ctx context.Context, clusterRef, namespace string) (*ClusterHealth, error) {
	log := log.FromContext(ctx)

	health := &ClusterHealth{
		Components:      make([]ComponentHealth, 0),
		Recommendations: make([]string, 0),
		LastHealthCheck: time.Now(),
	}

	// Get cluster resource
	cluster := &stormv1beta1.StormCluster{}
	if err := hm.Get(ctx, types.NamespacedName{
		Name:      clusterRef,
		Namespace: namespace,
	}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", clusterRef, err)
	}

	// Check Kubernetes resource health
	kubernetesHealth := hm.checkKubernetesResourceHealth(ctx, cluster)
	health.Components = append(health.Components, kubernetesHealth...)

	// Check Storm API connectivity and health
	stormHealth := hm.checkStormAPIHealth(ctx, cluster)
	health.Components = append(health.Components, stormHealth...)

	// Calculate topology capacity
	capacity, err := hm.calculateTopologyCapacity(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to calculate topology capacity")
	} else {
		health.TopologyCapacity = capacity
	}

	// Calculate overall status and readiness score
	health.OverallStatus, health.ReadinessScore = hm.calculateOverallHealth(health.Components)

	// Generate recommendations
	health.Recommendations = hm.generateRecommendations(health)

	return health, nil
}

// checkKubernetesResourceHealth checks the health of Kubernetes resources
func (hm *HealthMonitor) checkKubernetesResourceHealth(ctx context.Context, cluster *stormv1beta1.StormCluster) []ComponentHealth {
	components := make([]ComponentHealth, 0)

	// Check Nimbus health
	nimbusHealth := ComponentHealth{
		Component:    "Nimbus",
		LastChecked:  time.Now(),
		CheckDetails: make(map[string]interface{}),
	}

	var nimbusReplicas int32 = 1
	if cluster.Spec.Nimbus != nil && cluster.Spec.Nimbus.Replicas != nil {
		nimbusReplicas = *cluster.Spec.Nimbus.Replicas
	}

	if cluster.Status.NimbusReady >= nimbusReplicas {
		nimbusHealth.Status = HealthStatusHealthy
		nimbusHealth.Message = fmt.Sprintf("All %d Nimbus replicas are ready", nimbusReplicas)
	} else if cluster.Status.NimbusReady > 0 {
		nimbusHealth.Status = HealthStatusDegraded
		nimbusHealth.Message = fmt.Sprintf("Only %d of %d Nimbus replicas are ready", cluster.Status.NimbusReady, nimbusReplicas)
	} else {
		nimbusHealth.Status = HealthStatusUnhealthy
		nimbusHealth.Message = "No Nimbus replicas are ready"
	}

	nimbusHealth.CheckDetails["ready_replicas"] = cluster.Status.NimbusReady
	nimbusHealth.CheckDetails["desired_replicas"] = cluster.Spec.Nimbus.Replicas
	components = append(components, nimbusHealth)

	// Check Supervisor health
	supervisorHealth := ComponentHealth{
		Component:    "Supervisor",
		LastChecked:  time.Now(),
		CheckDetails: make(map[string]interface{}),
	}

	var supervisorReplicas int32 = 1
	if cluster.Spec.Supervisor != nil && cluster.Spec.Supervisor.Replicas != nil {
		supervisorReplicas = *cluster.Spec.Supervisor.Replicas
	}
	minSupervisors := supervisorReplicas / 2
	if minSupervisors == 0 {
		minSupervisors = 1
	}

	if cluster.Status.SupervisorReady >= supervisorReplicas {
		supervisorHealth.Status = HealthStatusHealthy
		supervisorHealth.Message = fmt.Sprintf("All %d Supervisor replicas are ready", supervisorReplicas)
	} else if cluster.Status.SupervisorReady >= minSupervisors {
		supervisorHealth.Status = HealthStatusDegraded
		supervisorHealth.Message = fmt.Sprintf("Only %d of %d Supervisor replicas are ready", cluster.Status.SupervisorReady, supervisorReplicas)
	} else {
		supervisorHealth.Status = HealthStatusUnhealthy
		supervisorHealth.Message = fmt.Sprintf("Insufficient Supervisor replicas: %d of %d ready (minimum %d)", cluster.Status.SupervisorReady, supervisorReplicas, minSupervisors)
	}

	supervisorHealth.CheckDetails["ready_replicas"] = cluster.Status.SupervisorReady
	supervisorHealth.CheckDetails["desired_replicas"] = cluster.Spec.Supervisor.Replicas
	supervisorHealth.CheckDetails["minimum_required"] = minSupervisors
	components = append(components, supervisorHealth)

	// Check UI health (if enabled)
	if cluster.Spec.UI.Enabled {
		uiHealth := ComponentHealth{
			Component:    "UI",
			LastChecked:  time.Now(),
			CheckDetails: make(map[string]interface{}),
		}

		if cluster.Status.UIReady >= 1 {
			uiHealth.Status = HealthStatusHealthy
			uiHealth.Message = "Storm UI is ready"
		} else {
			uiHealth.Status = HealthStatusUnhealthy
			uiHealth.Message = "Storm UI is not ready"
		}

		uiHealth.CheckDetails["ready_replicas"] = cluster.Status.UIReady
		components = append(components, uiHealth)
	}

	return components
}

// checkStormAPIHealth checks Storm API connectivity and health
func (hm *HealthMonitor) checkStormAPIHealth(ctx context.Context, cluster *stormv1beta1.StormCluster) []ComponentHealth {
	components := make([]ComponentHealth, 0)

	apiHealth := ComponentHealth{
		Component:    "StormAPI",
		LastChecked:  time.Now(),
		CheckDetails: make(map[string]interface{}),
	}

	// Try to get Storm client
	stormClient, err := hm.ClientManager.GetClient()
	if err != nil {
		apiHealth.Status = HealthStatusUnhealthy
		apiHealth.Message = fmt.Sprintf("Cannot connect to Storm API: %v", err)
		apiHealth.CheckDetails["error"] = err.Error()
		components = append(components, apiHealth)
		return components
	}

	// Test cluster info API
	clusterInfo, err := stormClient.GetClusterInfo(ctx)
	if err != nil {
		apiHealth.Status = HealthStatusUnhealthy
		apiHealth.Message = fmt.Sprintf("Storm API not responding: %v", err)
		apiHealth.CheckDetails["error"] = err.Error()
	} else {
		apiHealth.Status = HealthStatusHealthy
		apiHealth.Message = "Storm API is responding"
		apiHealth.CheckDetails["cluster_info"] = clusterInfo

		// TODO: Add more detailed API health checks
		// - Check topology list API
		// - Check supervisor list API
		// - Measure API response times
	}

	components = append(components, apiHealth)
	return components
}

// calculateTopologyCapacity calculates the cluster's capacity for topologies
func (hm *HealthMonitor) calculateTopologyCapacity(ctx context.Context, cluster *stormv1beta1.StormCluster) (*TopologyCapacity, error) {
	// Get all topologies in the same namespace
	topologyList := &stormv1beta1.StormTopologyList{}
	if err := hm.List(ctx, topologyList, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	// Count running topologies for this cluster
	runningTopologies := int32(0)
	for _, topology := range topologyList.Items {
		if topology.Spec.ClusterRef == cluster.Name && topology.Status.Phase == "Running" {
			runningTopologies++
		}
	}

	// Calculate total available slots
	var totalSlots int32
	if cluster.Spec.Supervisor != nil && cluster.Spec.Supervisor.Replicas != nil {
		totalSlots = *cluster.Spec.Supervisor.Replicas * cluster.Spec.Supervisor.SlotsPerSupervisor
	}

	// Calculate used slots (simplified - assume 1 slot per running topology for now)
	usedSlots := runningTopologies
	availableSlots := totalSlots - usedSlots

	// Calculate utilization
	utilizationPercent := float64(usedSlots) / float64(totalSlots) * 100
	if totalSlots == 0 {
		utilizationPercent = 0
	}

	// Estimate max topologies (conservative estimate)
	maxTopologies := totalSlots

	return &TopologyCapacity{
		MaxTopologies:      maxTopologies,
		RunningTopologies:  runningTopologies,
		AvailableSlots:     availableSlots,
		TotalSlots:         totalSlots,
		UtilizationPercent: utilizationPercent,
	}, nil
}

// calculateOverallHealth calculates overall health status and readiness score
func (hm *HealthMonitor) calculateOverallHealth(components []ComponentHealth) (HealthStatus, int) {
	if len(components) == 0 {
		return HealthStatusUnknown, 0
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	// Count component health statuses
	for _, component := range components {
		switch component.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		}
	}

	totalComponents := len(components)

	// Calculate readiness score (0-100)
	score := (healthyCount*100 + degradedCount*50) / totalComponents

	// Determine overall status
	var overallStatus HealthStatus
	if unhealthyCount > 0 {
		if healthyCount == 0 {
			overallStatus = HealthStatusUnhealthy
		} else {
			overallStatus = HealthStatusDegraded
		}
	} else if degradedCount > 0 {
		overallStatus = HealthStatusDegraded
	} else if healthyCount > 0 {
		overallStatus = HealthStatusHealthy
	} else {
		overallStatus = HealthStatusUnknown
	}

	return overallStatus, score
}

// generateRecommendations generates health and optimization recommendations
func (hm *HealthMonitor) generateRecommendations(health *ClusterHealth) []string {
	recommendations := make([]string, 0)

	// Check for unhealthy components
	for _, component := range health.Components {
		if component.Status == HealthStatusUnhealthy {
			recommendations = append(recommendations,
				fmt.Sprintf("Address %s health issues: %s", component.Component, component.Message))
		}
	}

	// Check utilization and capacity
	if health.TopologyCapacity != nil {
		if health.TopologyCapacity.UtilizationPercent > 80 {
			recommendations = append(recommendations,
				"High cluster utilization detected. Consider scaling up supervisors or optimizing topologies.")
		}

		if health.TopologyCapacity.AvailableSlots < 2 {
			recommendations = append(recommendations,
				"Low available capacity. Consider adding more supervisor nodes.")
		}
	}

	// Check readiness score
	if health.ReadinessScore < 70 {
		recommendations = append(recommendations,
			"Cluster readiness is below optimal. Review component health and resource allocation.")
	}

	// Add general recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Cluster appears healthy. Monitor regularly for optimal performance.")
	}

	return recommendations
}

// IsClusterReadyForTopology determines if cluster is ready for new topology deployment
func (hm *HealthMonitor) IsClusterReadyForTopology(ctx context.Context, clusterRef, namespace string) (bool, string, time.Duration) {
	health, err := hm.CheckClusterHealth(ctx, clusterRef, namespace)
	if err != nil {
		return false, fmt.Sprintf("Health check failed: %v", err), 60 * time.Second
	}

	// Check overall health
	if health.OverallStatus == HealthStatusUnhealthy {
		return false, "Cluster is unhealthy", 30 * time.Second
	}

	// Check readiness score
	if health.ReadinessScore < 60 {
		return false, fmt.Sprintf("Cluster readiness score too low: %d/100", health.ReadinessScore), 30 * time.Second
	}

	// Check capacity
	if health.TopologyCapacity != nil && health.TopologyCapacity.AvailableSlots < 1 {
		return false, "No available capacity for new topologies", 60 * time.Second
	}

	// All checks passed
	return true, "Cluster is ready for topology deployment", 0
}
