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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Storm cluster metrics
	StormClusterInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_cluster_info",
			Help: "Information about Storm cluster",
		},
		[]string{"cluster", "namespace", "version"},
	)

	StormClusterSupervisors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_cluster_supervisors_total",
			Help: "Total number of supervisors in the Storm cluster",
		},
		[]string{"cluster", "namespace"},
	)

	StormClusterSlots = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_cluster_slots_total",
			Help: "Total number of worker slots in the Storm cluster",
		},
		[]string{"cluster", "namespace", "state"},
	)

	// Storm topology metrics
	StormTopologyInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_info",
			Help: "Information about Storm topology",
		},
		[]string{"topology", "namespace", "cluster", "status"},
	)

	StormTopologyWorkers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_workers_total",
			Help: "Total number of workers for a topology",
		},
		[]string{"topology", "namespace", "cluster"},
	)

	StormTopologyExecutors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_executors_total",
			Help: "Total number of executors for a topology",
		},
		[]string{"topology", "namespace", "cluster"},
	)

	StormTopologyTasks = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_tasks_total",
			Help: "Total number of tasks for a topology",
		},
		[]string{"topology", "namespace", "cluster"},
	)

	StormTopologyUptime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_uptime_seconds",
			Help: "Uptime of the topology in seconds",
		},
		[]string{"topology", "namespace", "cluster"},
	)

	// Storm worker pool metrics
	StormWorkerPoolReplicas = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_worker_pool_replicas",
			Help: "Number of replicas in a worker pool",
		},
		[]string{"pool", "namespace", "topology", "state"},
	)

	// Controller operation metrics
	StormTopologySubmissions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_topology_submissions_total",
			Help: "Total number of topology submissions",
		},
		[]string{"namespace", "result"},
	)

	StormTopologyDeletions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_topology_deletions_total",
			Help: "Total number of topology deletions",
		},
		[]string{"namespace", "result"},
	)

	// Topology lifecycle metrics
	StormTopologyStateTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_topology_state_transitions_total",
			Help: "Total number of topology state transitions",
		},
		[]string{"namespace", "topology", "from", "to"},
	)

	StormTopologyStateTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storm_topology_state_duration_seconds",
			Help:    "Time spent in each topology state",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
		},
		[]string{"namespace", "topology", "state"},
	)

	// JAR download/extraction metrics
	StormTopologyJarDownloadTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storm_topology_jar_download_duration_seconds",
			Help:    "Time taken to download topology JAR",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~100s
		},
		[]string{"namespace", "topology", "source_type"},
	)

	StormTopologyJarSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storm_topology_jar_size_bytes",
			Help:    "Size of topology JAR files",
			Buckets: prometheus.ExponentialBuckets(1048576, 2, 10), // 1MB to ~1GB
		},
		[]string{"namespace", "topology"},
	)

	// Performance metrics
	StormTopologyLatency = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_latency_ms",
			Help: "Average latency of topology in milliseconds",
		},
		[]string{"topology", "namespace", "cluster"},
	)

	StormTopologyThroughput = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_throughput_tuples_per_second",
			Help: "Throughput of topology in tuples per second",
		},
		[]string{"topology", "namespace", "cluster", "component"},
	)

	StormTopologyErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_topology_errors_total",
			Help: "Total number of errors in topology",
		},
		[]string{"topology", "namespace", "cluster", "component", "error_type"},
	)

	// Resource usage metrics
	StormTopologyMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_memory_usage_bytes",
			Help: "Memory usage of topology workers",
		},
		[]string{"topology", "namespace", "cluster", "worker"},
	)

	StormTopologyCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "storm_topology_cpu_usage_cores",
			Help: "CPU usage of topology workers",
		},
		[]string{"topology", "namespace", "cluster", "worker"},
	)

	// Controller reconciliation metrics
	ReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storm_controller_reconciliation_duration_seconds",
			Help:    "Time taken to reconcile resources",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller", "namespace", "name", "result"},
	)

	ReconciliationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_controller_reconciliation_errors_total",
			Help: "Total number of reconciliation errors",
		},
		[]string{"controller", "namespace", "name", "error_type"},
	)

	// Storm API metrics
	StormAPIRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storm_api_requests_total",
			Help: "Total number of Storm API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	StormAPIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storm_api_request_duration_seconds",
			Help:    "Storm API request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		StormClusterInfo,
		StormClusterSupervisors,
		StormClusterSlots,
		StormTopologyInfo,
		StormTopologyWorkers,
		StormTopologyExecutors,
		StormTopologyTasks,
		StormTopologyUptime,
		StormWorkerPoolReplicas,
		StormTopologySubmissions,
		StormTopologyDeletions,
		StormTopologyStateTransitions,
		StormTopologyStateTime,
		StormTopologyJarDownloadTime,
		StormTopologyJarSize,
		StormTopologyLatency,
		StormTopologyThroughput,
		StormTopologyErrors,
		StormTopologyMemoryUsage,
		StormTopologyCPUUsage,
		ReconciliationDuration,
		ReconciliationErrors,
		StormAPIRequests,
		StormAPIRequestDuration,
	)
}
