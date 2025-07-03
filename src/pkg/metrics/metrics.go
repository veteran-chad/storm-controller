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
	)
}
