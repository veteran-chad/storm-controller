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
)

// CrossResourceMonitor monitors health across all Storm resources and coordinates recovery
type CrossResourceMonitor struct {
	client.Client
	HealthMonitor *HealthMonitor
}

// NewCrossResourceMonitor creates a new cross-resource monitor
func NewCrossResourceMonitor(client client.Client, healthMonitor *HealthMonitor) *CrossResourceMonitor {
	return &CrossResourceMonitor{
		Client:        client,
		HealthMonitor: healthMonitor,
	}
}

// SystemHealthStatus represents the overall health of the Storm system
type SystemHealthStatus struct {
	OverallHealth   HealthStatus
	Clusters        []ClusterHealth
	Topologies      []TopologyHealth
	WorkerPools     []WorkerPoolHealth
	SystemMetrics   *SystemMetrics
	HealthScore     int // 0-100
	CriticalIssues  []HealthIssue
	Recommendations []string
	LastChecked     time.Time
}

// TopologyHealth represents the health of a topology
type TopologyHealth struct {
	Name           string
	Namespace      string
	Status         HealthStatus
	Phase          string
	ClusterRef     string
	WorkerPoolRefs []string
	Metrics        *TopologyMetrics
	Issues         []HealthIssue
	LastChecked    time.Time
}

// WorkerPoolHealth represents the health of a worker pool
type WorkerPoolHealth struct {
	Name        string
	Namespace   string
	Status      HealthStatus
	Phase       string
	TopologyRef string
	ClusterRef  string
	Metrics     *WorkerPoolMetrics
	Issues      []HealthIssue
	LastChecked time.Time
}

// SystemMetrics represents system-wide metrics
type SystemMetrics struct {
	TotalClusters        int
	HealthyClusters      int
	TotalTopologies      int
	RunningTopologies    int
	FailedTopologies     int
	TotalWorkerPools     int
	ReadyWorkerPools     int
	SystemUtilization    float64
	AlertCount           int
	RecoveryActionsCount int
}

// TopologyMetrics represents topology-specific metrics
type TopologyMetrics struct {
	Workers           int32
	Executors         int32
	Tasks             int32
	Uptime            time.Duration
	ProcessedMessages int64
	FailedMessages    int64
	Latency           time.Duration
	Throughput        float64
}

// WorkerPoolMetrics represents worker pool metrics
type WorkerPoolMetrics struct {
	DesiredReplicas   int32
	ReadyReplicas     int32
	AvailableReplicas int32
	CPUUtilization    float64
	MemoryUtilization float64
	TasksProcessed    int64
	ErrorRate         float64
}

// HealthIssue represents a health issue with severity and recovery suggestions
type HealthIssue struct {
	Severity        IssueSeverity
	Type            IssueType
	Component       string
	Message         string
	Recommendation  string
	AutoRecoverable bool
	FirstSeen       time.Time
	LastSeen        time.Time
	Count           int
}

// IssueSeverity represents the severity of a health issue
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "Critical"
	SeverityHigh     IssueSeverity = "High"
	SeverityMedium   IssueSeverity = "Medium"
	SeverityLow      IssueSeverity = "Low"
	SeverityInfo     IssueSeverity = "Info"
)

// IssueType represents the type of health issue
type IssueType string

const (
	IssueTypeResource      IssueType = "Resource"
	IssueTypeConnectivity  IssueType = "Connectivity"
	IssueTypePerformance   IssueType = "Performance"
	IssueTypeConfiguration IssueType = "Configuration"
	IssueTypeDependency    IssueType = "Dependency"
	IssueTypeCapacity      IssueType = "Capacity"
)

// RecoveryAction represents an automated recovery action
type RecoveryAction struct {
	Type         RecoveryActionType
	Target       string
	Namespace    string
	Description  string
	ExecutedAt   time.Time
	Success      bool
	ErrorMessage string
}

// RecoveryActionType defines types of automated recovery actions
type RecoveryActionType string

const (
	RecoveryActionRestart    RecoveryActionType = "Restart"
	RecoveryActionScale      RecoveryActionType = "Scale"
	RecoveryActionReschedule RecoveryActionType = "Reschedule"
	RecoveryActionRepair     RecoveryActionType = "Repair"
	RecoveryActionAlert      RecoveryActionType = "Alert"
)

// CheckSystemHealth performs comprehensive health check across all Storm resources
func (crm *CrossResourceMonitor) CheckSystemHealth(ctx context.Context, namespace string) (*SystemHealthStatus, error) {
	log := log.FromContext(ctx)

	systemHealth := &SystemHealthStatus{
		Clusters:        make([]ClusterHealth, 0),
		Topologies:      make([]TopologyHealth, 0),
		WorkerPools:     make([]WorkerPoolHealth, 0),
		CriticalIssues:  make([]HealthIssue, 0),
		Recommendations: make([]string, 0),
		LastChecked:     time.Now(),
		SystemMetrics:   &SystemMetrics{},
	}

	// Check all clusters
	clusters, err := crm.getAllClusters(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get clusters: %w", err)
	}

	for _, cluster := range clusters {
		clusterHealth, err := crm.checkClusterHealth(ctx, &cluster)
		if err != nil {
			log.Error(err, "Failed to check cluster health", "cluster", cluster.Name)
			continue
		}
		systemHealth.Clusters = append(systemHealth.Clusters, *clusterHealth)
	}

	// Check all topologies
	topologies, err := crm.getAllTopologies(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get topologies: %w", err)
	}

	for _, topology := range topologies {
		topologyHealth, err := crm.checkTopologyHealth(ctx, &topology)
		if err != nil {
			log.Error(err, "Failed to check topology health", "topology", topology.Name)
			continue
		}
		systemHealth.Topologies = append(systemHealth.Topologies, *topologyHealth)
	}

	// Check all worker pools
	workerPools, err := crm.getAllWorkerPools(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker pools: %w", err)
	}

	for _, pool := range workerPools {
		poolHealth, err := crm.checkWorkerPoolHealth(ctx, &pool)
		if err != nil {
			log.Error(err, "Failed to check worker pool health", "pool", pool.Name)
			continue
		}
		systemHealth.WorkerPools = append(systemHealth.WorkerPools, *poolHealth)
	}

	// Calculate system metrics and overall health
	crm.calculateSystemMetrics(systemHealth)
	crm.identifyCriticalIssues(systemHealth)
	crm.generateSystemRecommendations(systemHealth)

	return systemHealth, nil
}

// checkClusterHealth checks the health of a specific cluster
func (crm *CrossResourceMonitor) checkClusterHealth(ctx context.Context, cluster *stormv1beta1.StormCluster) (*ClusterHealth, error) {
	// Use existing health monitor for detailed cluster health
	health, err := crm.HealthMonitor.CheckClusterHealth(ctx, cluster.Name, cluster.Namespace)
	if err != nil {
		return nil, err
	}

	// Add cross-resource specific health checks
	crm.addClusterCrossResourceChecks(ctx, cluster, health)

	return health, nil
}

// checkTopologyHealth checks the health of a specific topology
func (crm *CrossResourceMonitor) checkTopologyHealth(ctx context.Context, topology *stormv1beta1.StormTopology) (*TopologyHealth, error) {
	health := &TopologyHealth{
		Name:           topology.Name,
		Namespace:      topology.Namespace,
		Phase:          topology.Status.Phase,
		ClusterRef:     topology.Spec.ClusterRef,
		WorkerPoolRefs: make([]string, 0),
		Issues:         make([]HealthIssue, 0),
		LastChecked:    time.Now(),
		Metrics:        &TopologyMetrics{},
	}

	// Determine topology health status
	switch topology.Status.Phase {
	case "Running":
		health.Status = HealthStatusHealthy
	case "Failed", "Killed":
		health.Status = HealthStatusUnhealthy
	case "Pending", "Validating", "Downloading", "Submitting":
		health.Status = HealthStatusDegraded
	default:
		health.Status = HealthStatusUnknown
	}

	// Check topology dependencies
	if err := crm.checkTopologyDependencies(ctx, topology, health); err != nil {
		health.Issues = append(health.Issues, HealthIssue{
			Severity:        SeverityHigh,
			Type:            IssueTypeDependency,
			Component:       "Topology",
			Message:         fmt.Sprintf("Dependency check failed: %v", err),
			Recommendation:  "Verify cluster and worker pool dependencies are healthy",
			AutoRecoverable: false,
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
			Count:           1,
		})
	}

	// Check topology metrics (if available)
	crm.collectTopologyMetrics(ctx, topology, health)

	return health, nil
}

// checkWorkerPoolHealth checks the health of a specific worker pool
func (crm *CrossResourceMonitor) checkWorkerPoolHealth(ctx context.Context, pool *stormv1beta1.StormWorkerPool) (*WorkerPoolHealth, error) {
	health := &WorkerPoolHealth{
		Name:        pool.Name,
		Namespace:   pool.Namespace,
		Phase:       pool.Status.Phase,
		TopologyRef: pool.Spec.TopologyRef,
		ClusterRef:  pool.Spec.ClusterRef,
		Issues:      make([]HealthIssue, 0),
		LastChecked: time.Now(),
		Metrics:     &WorkerPoolMetrics{},
	}

	// Determine worker pool health status
	switch pool.Status.Phase {
	case "Ready":
		if pool.Status.ReadyReplicas >= pool.Spec.Replicas {
			health.Status = HealthStatusHealthy
		} else {
			health.Status = HealthStatusDegraded
		}
	case "Failed":
		health.Status = HealthStatusUnhealthy
	case "Creating", "Scaling", "Updating":
		health.Status = HealthStatusDegraded
	default:
		health.Status = HealthStatusUnknown
	}

	// Collect worker pool metrics
	health.Metrics.DesiredReplicas = pool.Spec.Replicas
	health.Metrics.ReadyReplicas = pool.Status.ReadyReplicas
	health.Metrics.AvailableReplicas = pool.Status.AvailableReplicas

	// Check for capacity issues
	if pool.Status.ReadyReplicas < pool.Spec.Replicas {
		health.Issues = append(health.Issues, HealthIssue{
			Severity:        SeverityMedium,
			Type:            IssueTypeCapacity,
			Component:       "WorkerPool",
			Message:         fmt.Sprintf("Ready replicas (%d) less than desired (%d)", pool.Status.ReadyReplicas, pool.Spec.Replicas),
			Recommendation:  "Check pod scheduling and resource availability",
			AutoRecoverable: true,
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
			Count:           1,
		})
	}

	return health, nil
}

// addClusterCrossResourceChecks adds cross-resource health checks for clusters
func (crm *CrossResourceMonitor) addClusterCrossResourceChecks(ctx context.Context, cluster *stormv1beta1.StormCluster, health *ClusterHealth) {
	// Check dependent topologies
	topologies, err := crm.getTopologiesForCluster(ctx, cluster.Name, cluster.Namespace)
	if err == nil {
		runningTopologies := 0
		failedTopologies := 0

		for _, topology := range topologies {
			switch topology.Status.Phase {
			case "Running":
				runningTopologies++
			case "Failed", "Killed":
				failedTopologies++
			}
		}

		if failedTopologies > 0 {
			health.Recommendations = append(health.Recommendations,
				fmt.Sprintf("Cluster has %d failed topologies that may need attention", failedTopologies))
		}
	}

	// Check dependent worker pools
	workerPools, err := crm.getWorkerPoolsForCluster(ctx, cluster.Name, cluster.Namespace)
	if err == nil {
		unhealthyPools := 0

		for _, pool := range workerPools {
			if pool.Status.Phase != "Ready" || pool.Status.ReadyReplicas < pool.Spec.Replicas {
				unhealthyPools++
			}
		}

		if unhealthyPools > 0 {
			health.Recommendations = append(health.Recommendations,
				fmt.Sprintf("Cluster has %d unhealthy worker pools", unhealthyPools))
		}
	}
}

// checkTopologyDependencies checks topology dependencies
func (crm *CrossResourceMonitor) checkTopologyDependencies(ctx context.Context, topology *stormv1beta1.StormTopology, health *TopologyHealth) error {
	// Check cluster dependency
	cluster := &stormv1beta1.StormCluster{}
	if err := crm.Get(ctx, types.NamespacedName{
		Name:      topology.Spec.ClusterRef,
		Namespace: topology.Namespace,
	}, cluster); err != nil {
		return fmt.Errorf("cluster %s not found", topology.Spec.ClusterRef)
	}

	if cluster.Status.Phase != "Running" {
		return fmt.Errorf("cluster %s is not running (current phase: %s)", topology.Spec.ClusterRef, cluster.Status.Phase)
	}

	// Find associated worker pools
	workerPools, err := crm.getWorkerPoolsForTopology(ctx, topology.Name, topology.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check worker pools: %w", err)
	}

	for _, pool := range workerPools {
		health.WorkerPoolRefs = append(health.WorkerPoolRefs, pool.Name)
		if pool.Status.Phase != "Ready" {
			health.Issues = append(health.Issues, HealthIssue{
				Severity:        SeverityMedium,
				Type:            IssueTypeDependency,
				Component:       "WorkerPool",
				Message:         fmt.Sprintf("Worker pool %s is not ready (phase: %s)", pool.Name, pool.Status.Phase),
				Recommendation:  "Check worker pool status and resolve any issues",
				AutoRecoverable: true,
				FirstSeen:       time.Now(),
				LastSeen:        time.Now(),
				Count:           1,
			})
		}
	}

	return nil
}

// collectTopologyMetrics collects topology-specific metrics
func (crm *CrossResourceMonitor) collectTopologyMetrics(ctx context.Context, topology *stormv1beta1.StormTopology, health *TopologyHealth) {
	// Collect basic metrics from topology spec
	if topology.Spec.Workers != nil {
		health.Metrics.Workers = topology.Spec.Workers.Replicas
	}

	// TODO: Integrate with Storm metrics API to get real-time metrics
	// This would require connecting to Storm UI API or JMX endpoints
	// For now, we'll use placeholder values
	health.Metrics.Uptime = time.Since(topology.CreationTimestamp.Time)
}

// calculateSystemMetrics calculates overall system metrics
func (crm *CrossResourceMonitor) calculateSystemMetrics(systemHealth *SystemHealthStatus) {
	metrics := systemHealth.SystemMetrics

	// Calculate cluster metrics
	metrics.TotalClusters = len(systemHealth.Clusters)
	for _, cluster := range systemHealth.Clusters {
		if cluster.OverallStatus == HealthStatusHealthy {
			metrics.HealthyClusters++
		}
	}

	// Calculate topology metrics
	metrics.TotalTopologies = len(systemHealth.Topologies)
	for _, topology := range systemHealth.Topologies {
		if topology.Status == HealthStatusHealthy && topology.Phase == "Running" {
			metrics.RunningTopologies++
		}
		if topology.Status == HealthStatusUnhealthy {
			metrics.FailedTopologies++
		}
	}

	// Calculate worker pool metrics
	metrics.TotalWorkerPools = len(systemHealth.WorkerPools)
	for _, pool := range systemHealth.WorkerPools {
		if pool.Status == HealthStatusHealthy {
			metrics.ReadyWorkerPools++
		}
	}

	// Calculate overall health score
	systemHealth.HealthScore = crm.calculateHealthScore(systemHealth)

	// Determine overall system health
	if systemHealth.HealthScore >= 90 {
		systemHealth.OverallHealth = HealthStatusHealthy
	} else if systemHealth.HealthScore >= 70 {
		systemHealth.OverallHealth = HealthStatusDegraded
	} else {
		systemHealth.OverallHealth = HealthStatusUnhealthy
	}
}

// calculateHealthScore calculates a 0-100 health score
func (crm *CrossResourceMonitor) calculateHealthScore(systemHealth *SystemHealthStatus) int {
	if systemHealth.SystemMetrics.TotalClusters == 0 {
		return 0
	}

	// Weight different components
	clusterWeight := 50.0
	topologyWeight := 30.0
	workerPoolWeight := 20.0

	// Calculate weighted scores
	clusterScore := 0.0
	if systemHealth.SystemMetrics.TotalClusters > 0 {
		clusterScore = float64(systemHealth.SystemMetrics.HealthyClusters) / float64(systemHealth.SystemMetrics.TotalClusters) * clusterWeight
	}

	topologyScore := 0.0
	if systemHealth.SystemMetrics.TotalTopologies > 0 {
		topologyScore = float64(systemHealth.SystemMetrics.RunningTopologies) / float64(systemHealth.SystemMetrics.TotalTopologies) * topologyWeight
	}

	workerPoolScore := 0.0
	if systemHealth.SystemMetrics.TotalWorkerPools > 0 {
		workerPoolScore = float64(systemHealth.SystemMetrics.ReadyWorkerPools) / float64(systemHealth.SystemMetrics.TotalWorkerPools) * workerPoolWeight
	}

	totalScore := clusterScore + topologyScore + workerPoolScore

	// Deduct points for critical issues
	criticalPenalty := float64(len(systemHealth.CriticalIssues)) * 5.0
	totalScore = totalScore - criticalPenalty

	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > 100 {
		totalScore = 100
	}

	return int(totalScore)
}

// identifyCriticalIssues identifies critical issues across the system
func (crm *CrossResourceMonitor) identifyCriticalIssues(systemHealth *SystemHealthStatus) {
	// Check for critical cluster issues
	for _, cluster := range systemHealth.Clusters {
		if cluster.OverallStatus == HealthStatusUnhealthy {
			systemHealth.CriticalIssues = append(systemHealth.CriticalIssues, HealthIssue{
				Severity:        SeverityCritical,
				Type:            IssueTypeResource,
				Component:       "Cluster",
				Message:         fmt.Sprintf("Cluster %s is unhealthy", cluster.Components[0].Component),
				Recommendation:  "Investigate cluster components and resolve issues immediately",
				AutoRecoverable: false,
				FirstSeen:       time.Now(),
				LastSeen:        time.Now(),
				Count:           1,
			})
		}
	}

	// Check for cascade failures
	if systemHealth.SystemMetrics.FailedTopologies > systemHealth.SystemMetrics.RunningTopologies {
		systemHealth.CriticalIssues = append(systemHealth.CriticalIssues, HealthIssue{
			Severity:        SeverityCritical,
			Type:            IssueTypePerformance,
			Component:       "System",
			Message:         "More topologies failing than running - potential cascade failure",
			Recommendation:  "Investigate root cause and implement emergency recovery procedures",
			AutoRecoverable: false,
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
			Count:           1,
		})
	}
}

// generateSystemRecommendations generates system-wide recommendations
func (crm *CrossResourceMonitor) generateSystemRecommendations(systemHealth *SystemHealthStatus) {
	// Performance recommendations
	if systemHealth.HealthScore < 80 {
		systemHealth.Recommendations = append(systemHealth.Recommendations,
			"System health score is below optimal. Review component health and address issues.")
	}

	// Capacity recommendations
	totalCapacity := 0
	usedCapacity := 0
	for _, cluster := range systemHealth.Clusters {
		if cluster.TopologyCapacity != nil {
			totalCapacity += int(cluster.TopologyCapacity.TotalSlots)
			usedCapacity += int(cluster.TopologyCapacity.TotalSlots - cluster.TopologyCapacity.AvailableSlots)
		}
	}

	if totalCapacity > 0 {
		utilization := float64(usedCapacity) / float64(totalCapacity) * 100
		if utilization > 80 {
			systemHealth.Recommendations = append(systemHealth.Recommendations,
				"High cluster utilization detected. Consider scaling up cluster capacity.")
		}
	}

	// Reliability recommendations
	if len(systemHealth.CriticalIssues) > 0 {
		systemHealth.Recommendations = append(systemHealth.Recommendations,
			fmt.Sprintf("System has %d critical issues requiring immediate attention.", len(systemHealth.CriticalIssues)))
	}
}

// Helper methods for resource retrieval
func (crm *CrossResourceMonitor) getAllClusters(ctx context.Context, namespace string) ([]stormv1beta1.StormCluster, error) {
	clusterList := &stormv1beta1.StormClusterList{}
	if err := crm.List(ctx, clusterList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return clusterList.Items, nil
}

func (crm *CrossResourceMonitor) getAllTopologies(ctx context.Context, namespace string) ([]stormv1beta1.StormTopology, error) {
	topologyList := &stormv1beta1.StormTopologyList{}
	if err := crm.List(ctx, topologyList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return topologyList.Items, nil
}

func (crm *CrossResourceMonitor) getAllWorkerPools(ctx context.Context, namespace string) ([]stormv1beta1.StormWorkerPool, error) {
	poolList := &stormv1beta1.StormWorkerPoolList{}
	if err := crm.List(ctx, poolList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return poolList.Items, nil
}

func (crm *CrossResourceMonitor) getTopologiesForCluster(ctx context.Context, clusterName, namespace string) ([]stormv1beta1.StormTopology, error) {
	topologies, err := crm.getAllTopologies(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var filtered []stormv1beta1.StormTopology
	for _, topology := range topologies {
		if topology.Spec.ClusterRef == clusterName {
			filtered = append(filtered, topology)
		}
	}
	return filtered, nil
}

func (crm *CrossResourceMonitor) getWorkerPoolsForCluster(ctx context.Context, clusterName, namespace string) ([]stormv1beta1.StormWorkerPool, error) {
	pools, err := crm.getAllWorkerPools(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var filtered []stormv1beta1.StormWorkerPool
	for _, pool := range pools {
		if pool.Spec.ClusterRef == clusterName {
			filtered = append(filtered, pool)
		}
	}
	return filtered, nil
}

func (crm *CrossResourceMonitor) getWorkerPoolsForTopology(ctx context.Context, topologyName, namespace string) ([]stormv1beta1.StormWorkerPool, error) {
	pools, err := crm.getAllWorkerPools(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var filtered []stormv1beta1.StormWorkerPool
	for _, pool := range pools {
		if pool.Spec.TopologyRef == topologyName {
			filtered = append(filtered, pool)
		}
	}
	return filtered, nil
}
