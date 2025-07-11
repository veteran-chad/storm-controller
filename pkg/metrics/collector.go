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

package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/storm"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TopologyMetricsCollector collects metrics from Storm API
type TopologyMetricsCollector struct {
	stormClient storm.Client
	interval    time.Duration
}

// NewTopologyMetricsCollector creates a new metrics collector
func NewTopologyMetricsCollector(client storm.Client, interval time.Duration) *TopologyMetricsCollector {
	return &TopologyMetricsCollector{
		stormClient: client,
		interval:    interval,
	}
}

// Start begins periodic metric collection
func (c *TopologyMetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectMetrics(ctx)
		}
	}
}

// collectMetrics collects metrics from Storm API
func (c *TopologyMetricsCollector) collectMetrics(ctx context.Context) {
	log := log.FromContext(ctx)

	// Get cluster info
	clusterInfo, err := c.stormClient.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Failed to get cluster info")
		return
	}

	// Update cluster metrics
	StormClusterSupervisors.WithLabelValues(
		"default", // cluster name
		"default", // namespace
	).Set(float64(clusterInfo.Supervisors))

	StormClusterSlots.WithLabelValues(
		"default",
		"default",
		"total",
	).Set(float64(clusterInfo.TotalSlots))

	StormClusterSlots.WithLabelValues(
		"default",
		"default",
		"used",
	).Set(float64(clusterInfo.UsedSlots))

	StormClusterSlots.WithLabelValues(
		"default",
		"default",
		"free",
	).Set(float64(clusterInfo.TotalSlots - clusterInfo.UsedSlots))

	// Get all topologies
	topologies, err := c.stormClient.ListTopologies(ctx)
	if err != nil {
		log.Error(err, "Failed to list topologies")
		return
	}

	// Collect metrics for each topology
	for _, topology := range topologies {
		c.collectTopologyMetrics(ctx, topology.Name)
	}
}

// collectTopologyMetrics collects metrics for a specific topology
func (c *TopologyMetricsCollector) collectTopologyMetrics(ctx context.Context, topologyName string) {
	log := log.FromContext(ctx)

	// Get detailed topology info
	topologyInfo, err := c.stormClient.GetTopology(ctx, topologyName)
	if err != nil {
		log.Error(err, "Failed to get topology info", "topology", topologyName)
		return
	}

	// Update topology metrics
	StormTopologyWorkers.WithLabelValues(
		topologyName,
		"default", // namespace
		"default", // cluster
	).Set(float64(topologyInfo.Workers))

	StormTopologyExecutors.WithLabelValues(
		topologyName,
		"default",
		"default",
	).Set(float64(topologyInfo.Executors))

	StormTopologyTasks.WithLabelValues(
		topologyName,
		"default",
		"default",
	).Set(float64(topologyInfo.Tasks))

	// Calculate uptime
	if topologyInfo.LaunchTime != "" {
		launchTime, err := time.Parse(time.RFC3339, topologyInfo.LaunchTime)
		if err == nil {
			uptime := time.Since(launchTime).Seconds()
			StormTopologyUptime.WithLabelValues(
				topologyName,
				"default",
				"default",
			).Set(uptime)
		}
	}

	// Set topology status
	StormTopologyInfo.WithLabelValues(
		topologyName,
		"default",
		"default",
		topologyInfo.Status,
	).Set(1)
}

// CollectTopologySubmissionMetrics records topology submission metrics
func CollectTopologySubmissionMetrics(topology *stormv1beta1.StormTopology, result string, duration time.Duration) {
	// Record submission result
	StormTopologySubmissions.WithLabelValues(
		topology.Namespace,
		result,
	).Inc()

	// Record state transition
	StormTopologyStateTransitions.WithLabelValues(
		topology.Namespace,
		topology.Name,
		"Submitting",
		"Running",
	).Inc()

	// Record submission duration
	StormTopologyStateTime.WithLabelValues(
		topology.Namespace,
		topology.Name,
		"Submitting",
	).Observe(duration.Seconds())
}

// CollectJARDownloadMetrics records JAR download metrics
func CollectJARDownloadMetrics(topology *stormv1beta1.StormTopology, sourceType string, size int64, duration time.Duration) {
	// Record download time
	StormTopologyJarDownloadTime.WithLabelValues(
		topology.Namespace,
		topology.Name,
		sourceType,
	).Observe(duration.Seconds())

	// Record JAR size
	StormTopologyJarSize.WithLabelValues(
		topology.Namespace,
		topology.Name,
	).Observe(float64(size))
}

// CollectReconciliationMetrics records controller reconciliation metrics
func CollectReconciliationMetrics(controller, namespace, name, result string, duration time.Duration, err error) {
	// Record reconciliation duration
	ReconciliationDuration.WithLabelValues(
		controller,
		namespace,
		name,
		result,
	).Observe(duration.Seconds())

	// Record errors
	if err != nil {
		errorType := "unknown"
		if storm.IsNotFoundError(err) {
			errorType = "not_found"
		} else if storm.IsConnectionError(err) {
			errorType = "connection"
		} else if storm.IsAuthError(err) {
			errorType = "auth"
		}

		ReconciliationErrors.WithLabelValues(
			controller,
			namespace,
			name,
			errorType,
		).Inc()
	}
}

// CollectAPIMetrics records Storm API request metrics
func CollectAPIMetrics(method, endpoint string, statusCode int, duration time.Duration) {
	// Record request count
	StormAPIRequests.WithLabelValues(
		method,
		endpoint,
		fmt.Sprintf("%d", statusCode),
	).Inc()

	// Record request duration
	StormAPIRequestDuration.WithLabelValues(
		method,
		endpoint,
	).Observe(duration.Seconds())
}

// RecordStateTransition records a topology state transition
func RecordStateTransition(namespace, topology, from, to string) {
	StormTopologyStateTransitions.WithLabelValues(
		namespace,
		topology,
		from,
		to,
	).Inc()
}

// StartStateTimer starts timing how long a topology stays in a state
type StateTimer struct {
	namespace string
	topology  string
	state     string
	startTime time.Time
}

// NewStateTimer creates a new state timer
func NewStateTimer(namespace, topology, state string) *StateTimer {
	return &StateTimer{
		namespace: namespace,
		topology:  topology,
		state:     state,
		startTime: time.Now(),
	}
}

// Stop stops the timer and records the duration
func (t *StateTimer) Stop() {
	duration := time.Since(t.startTime)
	StormTopologyStateTime.WithLabelValues(
		t.namespace,
		t.topology,
		t.state,
	).Observe(duration.Seconds())
}