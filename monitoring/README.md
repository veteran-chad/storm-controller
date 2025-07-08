# Storm Controller Monitoring

This directory contains monitoring configurations for the Storm Controller, including Prometheus metrics, Grafana dashboards, and alerts.

## Overview

The Storm Controller exposes comprehensive metrics about:
- Storm cluster health and capacity
- Topology lifecycle and performance
- Controller operations and errors
- Resource usage and API performance

## Metrics

### Cluster Metrics
- `storm_cluster_info` - Basic cluster information
- `storm_cluster_supervisors_total` - Number of supervisors
- `storm_cluster_slots_total` - Worker slots by state (free/used)

### Topology Metrics
- `storm_topology_info` - Topology status information
- `storm_topology_workers_total` - Number of workers per topology
- `storm_topology_state_transitions_total` - State transition counter
- `storm_topology_latency_ms` - Average topology latency
- `storm_topology_throughput_tuples_per_second` - Tuple processing rate
- `storm_topology_errors_total` - Error counter by component

### Controller Metrics
- `storm_controller_reconciliation_duration_seconds` - Reconciliation time histogram
- `storm_controller_reconciliation_errors_total` - Reconciliation error counter
- `storm_topology_submissions_total` - Topology submission counter
- `storm_topology_jar_download_duration_seconds` - JAR download time

## Setup

### 1. Enable Metrics in Storm Operator

```yaml
# values.yaml
operator:
  metrics:
    enabled: true
    serviceMonitor:
      enabled: true  # If using Prometheus Operator
```

### 2. Deploy Prometheus

If using Prometheus Operator:

```bash
# Install Prometheus Operator
kubectl create namespace monitoring
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring
```

If using standalone Prometheus, add the Storm Controller as a scrape target:

```yaml
# prometheus.yaml
scrape_configs:
  - job_name: 'storm-controller'
    kubernetes_sd_configs:
      - role: endpoints
        namespaces:
          names:
            - storm-operator
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name]
        regex: storm-operator-operator
        action: keep
      - source_labels: [__meta_kubernetes_endpoint_port_name]
        regex: metrics
        action: keep
```

### 3. Import Grafana Dashboards

1. Access Grafana (if using kube-prometheus-stack):
   ```bash
   kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
   ```

2. Login (default: admin/prom-operator)

3. Import dashboards:
   - Go to Dashboards → Import
   - Upload `grafana/dashboards/storm-overview.json`
   - Upload `grafana/dashboards/storm-topology-details.json`

### 4. Configure Alerts

Apply the alert rules:

```bash
kubectl apply -f prometheus/alerts/storm-alerts.yaml
```

Or if using Prometheus Operator, create a PrometheusRule:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: storm-alerts
  namespace: storm-operator
  labels:
    prometheus: kube-prometheus
spec:
  groups:
    # Copy content from prometheus/alerts/storm-alerts.yaml
```

## Dashboard Screenshots

### Storm Overview Dashboard
- Cluster health status
- Available slots and supervisors
- Active topologies list
- Submission/deletion rates
- Controller performance

### Storm Topology Details Dashboard
- Topology state transitions
- Resource usage (CPU/Memory)
- Performance metrics (latency/throughput)
- Error rates by component
- JAR download times

## Alert Examples

### Critical Alerts
- `StormClusterDown` - Storm cluster unreachable
- `StormNimbusNotReady` - Nimbus not available
- `StormTopologyFailed` - Topology in failed state
- `StormControllerDown` - Controller not running

### Warning Alerts
- `StormNoAvailableSlots` - No free worker slots
- `StormTopologyHighLatency` - Latency > 1000ms
- `StormTopologyHighMemoryUsage` - Memory > 90%
- `StormControllerReconciliationSlow` - Reconciliation > 30s

## Metrics Collection from Storm

The controller can collect additional metrics from Storm's metrics system:

```go
// In your topology controller
func (r *StormTopologyReconciler) collectTopologyMetrics(ctx context.Context, topology *stormv1beta1.StormTopology) {
    // Get topology metrics from Storm API
    topologyMetrics, err := r.StormClient.GetTopologyMetrics(ctx, topology.Spec.Topology.Name)
    if err != nil {
        return
    }
    
    // Update Prometheus metrics
    metrics.StormTopologyLatency.WithLabelValues(
        topology.Name,
        topology.Namespace,
        topology.Spec.ClusterRef,
    ).Set(topologyMetrics.Latency)
    
    // Update throughput for each component
    for component, throughput := range topologyMetrics.ComponentThroughput {
        metrics.StormTopologyThroughput.WithLabelValues(
            topology.Name,
            topology.Namespace,
            topology.Spec.ClusterRef,
            component,
        ).Set(throughput)
    }
}
```

## Troubleshooting

### No Metrics Appearing
1. Check if the operator pod is running:
   ```bash
   kubectl get pods -n storm-operator
   ```

2. Verify metrics endpoint is accessible:
   ```bash
   kubectl port-forward -n storm-operator deployment/storm-operator-operator 8080:8080
   curl http://localhost:8080/metrics
   ```

3. Check ServiceMonitor is created:
   ```bash
   kubectl get servicemonitor -n storm-operator
   ```

### Missing Dashboards Data
1. Verify Prometheus is scraping metrics:
   - Go to Prometheus UI → Targets
   - Look for `storm-controller` job

2. Check metric names in Prometheus:
   - Go to Prometheus UI → Graph
   - Start typing `storm_` to see available metrics

3. Ensure dashboard variables are set correctly:
   - Check the `DS_PROMETHEUS` variable points to your Prometheus datasource

### Alerts Not Firing
1. Check PrometheusRule is loaded:
   ```bash
   kubectl get prometheusrule -A
   ```

2. Verify alert rules in Prometheus:
   - Go to Prometheus UI → Alerts
   - Check if Storm alerts are listed

3. Test alert conditions:
   ```bash
   # Example: Check if any topology is in failed state
   kubectl exec -n monitoring prometheus-0 -- promtool query instant \
     'storm_topology_info{status="Failed"}'
   ```

## Custom Metrics

To add custom metrics:

1. Define the metric in `pkg/metrics/metrics.go`:
   ```go
   MyCustomMetric = prometheus.NewGaugeVec(
       prometheus.GaugeOpts{
           Name: "storm_my_custom_metric",
           Help: "Description of my metric",
       },
       []string{"label1", "label2"},
   )
   ```

2. Register it in the init function

3. Update the metric in your controller:
   ```go
   metrics.MyCustomMetric.WithLabelValues("value1", "value2").Set(123)
   ```

4. Add to dashboards and alerts as needed