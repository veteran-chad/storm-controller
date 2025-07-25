# Apache Storm Helm Chart for Kubernetes

A production-ready Helm chart for deploying Apache Storm on Kubernetes with comprehensive features including high availability, security, monitoring, and autoscaling.

## Features

- **High Availability**: Multi-Nimbus setup with leader election via Zookeeper
- **Security**: RBAC, Pod Security Contexts, Network Policies, and TLS support
- **Monitoring**: Prometheus metrics, ServiceMonitor, PrometheusRules, and Grafana dashboards
- **Auto-scaling**: HPA support for supervisors with configurable metrics
- **Flexible Deployment**: Support for both embedded and external Zookeeper
- **Memory Management**: Automatic memory calculation for optimal JVM and worker settings
- **Production Ready**: PodDisruptionBudgets, resource quotas, and comprehensive health checks
- **Storm Controller Integration**: Optional CRD installation and controller deployment

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- PV provisioner support (for persistence)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) (optional, for monitoring)
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server) (optional, for HPA)
- [cert-manager](https://cert-manager.io/) (optional, for TLS)
- [NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/) (optional, for ingress)

## Quick Start

### Installation

1. **Basic Installation**
```bash
helm install my-storm . --namespace storm-system --create-namespace
```

2. **Development Installation with Local Values**
```bash
helm install my-storm . -f storm-local-values.yaml --namespace storm-system --create-namespace
```

3. **Production Installation**
```bash
helm install storm-prod . -f values-production.yaml --namespace storm-production --create-namespace
```

### Accessing Storm UI

1. **Port Forwarding (Development)**
```bash
kubectl port-forward svc/my-storm-ui 8080:8080 -n storm-system
# Access at http://localhost:8080
```

2. **Ingress (Production)**
```bash
# Ensure ingress is enabled in values
ui:
  ingress:
    enabled: true
    hostname: storm.example.com
    tls: true
```

### Deploying Topologies

1. **Using Storm CLI**
```bash
# Port-forward to Nimbus
kubectl port-forward svc/my-storm-nimbus 6627:6627 -n storm-system

# Submit topology
storm jar my-topology.jar com.example.MyTopology my-topology
```

2. **Using Storm Controller (Recommended)**
```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount
  namespace: storm-system
spec:
  jarSource:
    url: https://example.com/storm-starter-2.8.1.jar
  mainClass: org.apache.storm.starter.WordCountTopology
  args:
    - wordcount
  version: "1.0.0"
```

## Configuration

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nimbus.replicaCount` | Number of Nimbus nodes (3 for HA) | `1` |
| `supervisor.replicaCount` | Number of Supervisor nodes | `1` |
| `supervisor.slotsPerSupervisor` | Worker slots per supervisor | `4` |
| `supervisor.memoryConfig.mode` | Memory configuration mode: "auto" or "manual" | `"auto"` |
| `supervisor.memoryConfig.memoryPerWorker` | Memory per worker in auto mode | `"1Gi"` |
| `ui.enabled` | Enable Storm UI | `true` |
| `metrics.enabled` | Enable Prometheus metrics | `false` |
| `networkPolicy.enabled` | Enable network policies | `false` |
| `podSecurityContext.enabled` | Enable pod security context | `true` |

### Production Features

#### Security Configuration
```yaml
# Enable all security features
serviceAccount:
  create: true

rbac:
  create: true

podSecurityContext:
  enabled: true
  runAsUser: 1000
  runAsNonRoot: true

containerSecurityContext:
  enabled: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]

networkPolicy:
  enabled: true
```

#### High Availability
```yaml
# HA Nimbus setup
nimbus:
  replicaCount: 3
  pdb:
    create: true
    minAvailable: 2

# Supervisor availability
supervisor:
  pdb:
    create: true
    maxUnavailable: 1
```

#### Monitoring
```yaml
# Enable full monitoring stack
metrics:
  enabled: true
  serviceName: "storm-metrics"
  serviceVersion: "1.0.0"
  environment: "production"
  
  # Prometheus integration
  prometheus:
    scrape: true
    path: "/metrics"
  
  # OpenTelemetry integration
  otel:
    enabled: true
  
  # Datadog integration
  datadog:
    enabled: true
    scrapeLogs: true  # Adds log collection annotations
  
  # Metrics exporter configuration
  exporter:
    port: 9090
    logLevel: "INFO"
    enableDetailedMetrics: true
    enableComponentMetrics: true
    
  # ServiceMonitor for Prometheus Operator
  serviceMonitor:
    enabled: true
  
  # PrometheusRule for alerts
  prometheusRule:
    enabled: true
```

#### Autoscaling
```yaml
# HPA for supervisors
supervisor:
  hpa:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPU: 70      # CPU utilization percentage
    targetMemory: 80   # Memory utilization percentage
    # Optional custom metrics (requires metrics-server)
    metrics:
      - type: Pods
        pods:
          metric:
            name: storm_slots_used_ratio
          target:
            type: AverageValue
            averageValue: "0.8"
    # Scaling behavior (optional)
    behavior:
      scaleUp:
        stabilizationWindowSeconds: 120
        policies:
        - type: Percent
          value: 100
          periodSeconds: 60
      scaleDown:
        stabilizationWindowSeconds: 300
        policies:
        - type: Percent
          value: 10
          periodSeconds: 60
```

### Memory Configuration

The chart supports two memory configuration modes:

#### Auto Mode (Recommended)
Automatically calculates container resources based on worker requirements:

```yaml
supervisor:
  memoryConfig:
    mode: "auto"
    memoryPerWorker: "1Gi"        # Memory per worker slot
    memoryOverheadPercent: 25     # JVM overhead (metaspace, direct memory)
    cpuPerWorker: "1"             # CPU cores per worker
  slotsPerSupervisor: 4           # Number of workers per supervisor
```

With these settings:
- Container memory = 4 workers × 1Gi × 1.25 = 5Gi
- Worker heap = 1Gi × 0.8 = 819MB (80% of worker memory)
- Supervisor capacity = 4096MB (reported to Nimbus)

#### Manual Mode
Full control over memory settings:

```yaml
supervisor:
  memoryConfig:
    mode: "manual"
  resources:
    requests:
      memory: "29Gi"
      cpu: "3"
    limits:
      memory: "29Gi" 
      cpu: "3"
  extraConfig:
    supervisor.memory.capacity.mb: 28600
    supervisor.cpu.capacity: 300
    worker.heap.memory.mb: 768
```

### Storm Configuration

The chart supports two configuration methods:

#### 1. Environment Variables (Recommended)

All Storm configuration is automatically converted to environment variables and stored in a ConfigMap for easy updates:

```yaml
# Cluster-wide configuration
cluster:
  extraConfig:
    storm.log.level: "INFO"
    topology.workers: 2

# Component-specific configuration
nimbus:
  extraConfig:
    nimbus.childopts: "-Xmx2048m"
    nimbus.task.timeout.secs: 30
```

These are converted to environment variables (e.g., `STORM_NIMBUS__CHILDOPTS`) and stored in the `<release>-env` ConfigMap.

#### 2. Custom storm.yaml (Optional)

For complex configurations or when migrating existing storm.yaml files:

```yaml
cluster:
  stormYaml: |
    storm.zookeeper.servers:
      - "zookeeper"
    nimbus.seeds:
      - "nimbus"
    storm.log.dir: "/logs"
    storm.local.dir: "/data"
    ui.port: 8080
    # Any other Storm configuration...
```

This creates a ConfigMap with your storm.yaml that is mounted at `/conf/storm.yaml` in all containers.

### Logging Configuration

The new Storm container supports flexible logging through the `LOG_FORMAT` environment variable:

```yaml
# Set logging format per component
ui:
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"  # Options: text, json, or custom format

# JSON logging includes structured fields for better observability:
# - service, environment, version (from container metadata)
# - hostname/pod (Kubernetes pod name)
# - topology (for worker logs)
# - worker_port (for worker logs)
```

When using Datadog:
```yaml
metrics:
  datadog:
    enabled: true
    scrapeLogs: true  # Automatically adds Datadog log annotations
```

### External Zookeeper

```yaml
externalZookeeper:
  enabled: true
  servers:
    - zookeeper-0.zookeeper:2181
    - zookeeper-1.zookeeper:2181
    - zookeeper-2.zookeeper:2181
```

## Integrations

### OpenTelemetry

When OTEL is enabled, the following environment variables are set on the metrics exporter:
- `OTEL_SERVICE_NAME`: Set to `metrics.serviceName`
- `OTEL_RESOURCE_ATTRIBUTES`: Includes service version and environment

### Datadog

The chart provides comprehensive Datadog integration:

1. **Unified Service Tagging**: All pods are labeled with:
   - `tags.datadoghq.com/env`
   - `tags.datadoghq.com/service`
   - `tags.datadoghq.com/version`

2. **Environment Variables**: Datadog environment variables are injected:
   - `DD_ENV`: From pod labels
   - `DD_SERVICE`: From pod labels
   - `DD_VERSION`: From pod labels

3. **Log Collection**: When `metrics.datadog.scrapeLogs` is enabled:
   - Adds `ad.datadoghq.com/<container>.logs` annotations
   - Configures log source as "storm"
   - Sets service name per component (e.g., "storm-metrics-nimbus")

## Advanced Usage

### Custom Storm Configuration

```yaml
stormConfig:
  nimbus.seeds: ["nimbus-0", "nimbus-1", "nimbus-2"]
  topology.acker.executors: 4
  topology.max.spout.pending: 1000
  storm.messaging.netty.buffer_size: 5242880
```

### Persistent Storage

```yaml
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 50Gi
```

### Ingress with TLS

```yaml
ui:
  ingress:
    enabled: true
    hostname: storm.example.com
    tls: true
    annotations:
      cert-manager.io/cluster-issuer: "letsencrypt-prod"
      nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
```

### Pod Placement

```yaml
supervisor:
  nodeSelector:
    node-role.kubernetes.io/worker: "true"
  
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/component: supervisor
        topologyKey: kubernetes.io/hostname
```

## Monitoring and Alerts

### Available Metrics

The metrics exporter collects comprehensive Storm cluster metrics:

**Cluster Metrics:**
- `storm_cluster_slots_total`: Total supervisor slots
- `storm_cluster_slots_used`: Used supervisor slots  
- `storm_cluster_executors_total`: Total executors
- `storm_cluster_tasks_total`: Total tasks

**Topology Metrics:**
- `storm_topology_uptime_seconds`: Topology uptime
- `storm_topology_assigned_executors`: Executors per topology
- `storm_topology_assigned_tasks`: Tasks per topology
- `storm_topology_message_timeout_seconds`: Message timeout configuration

**Component Metrics (when `enableComponentMetrics: true`):**
- `storm_bolt_executed`: Messages executed by bolt
- `storm_bolt_acked`: Messages acknowledged by bolt
- `storm_bolt_failed`: Messages failed by bolt
- `storm_bolt_execute_latency_ms`: Bolt execution latency
- `storm_bolt_process_latency_ms`: Bolt process latency
- `storm_bolt_capacity`: Bolt capacity (0-1, higher means busier)
- `storm_spout_emitted`: Messages emitted by spout
- `storm_spout_acked`: Messages acknowledged
- `storm_spout_failed`: Messages failed
- `storm_spout_complete_latency_ms`: Spout complete latency

### Pre-configured Alerts

The chart includes Prometheus alerts:

- **StormClusterDown**: No Storm metrics for 5 minutes
- **StormHighSlotUsage**: >90% slot utilization
- **StormNimbusUnavailable**: Nimbus unreachable
- **StormSupervisorDown**: Supervisor failures

Access metrics at: `http://<storm-ui>:8080/api/v1/` (Storm API) or `http://<metrics-exporter>:9090/metrics` (Prometheus format)

## Troubleshooting

### Common Issues

1. **Supervisors not connecting to Nimbus**
   - Check network policies
   - Verify Zookeeper connectivity
   - Check service DNS resolution

2. **High memory usage**
   - Enable autoMemory configuration
   - Review JVM options
   - Check topology resource allocations

3. **Topology submission failures**
   - Verify sufficient supervisor slots
   - Check Nimbus logs
   - Ensure JAR accessibility

### Debugging Commands

```bash
# Check pod status
kubectl get pods -n storm-system

# View Nimbus logs
kubectl logs -f deployment/my-storm-nimbus -n storm-system

# Check supervisor slots
kubectl exec deployment/my-storm-supervisor -n storm-system -- storm list

# Verify metrics
curl http://localhost:9102/metrics | grep storm_
```

## Production Deployment

For detailed production deployment instructions, see [PRODUCTION.md](PRODUCTION.md).

Key considerations:
- Use external Zookeeper for production
- Enable all security features
- Configure resource limits appropriately
- Set up monitoring and alerting
- Use persistent storage for Nimbus
- Configure PodDisruptionBudgets

## Upgrading

1. **Review breaking changes in release notes**
2. **Test in staging environment first**
3. **Perform rolling upgrade:**

```bash
helm upgrade my-storm . \
  -f my-values.yaml \
  --namespace storm-system
```

## Uninstalling

```bash
# Remove the release
helm uninstall my-storm -n storm-system

# Clean up CRDs if installed
kubectl delete crd stormclusters.storm.apache.org
kubectl delete crd stormtopologies.storm.apache.org
```

## Contributing

Please see the [Storm Kubernetes Controller](https://github.com/apache/storm-kubernetes) repository for contribution guidelines.

## License

Copyright 2024 The Apache Software Foundation

Licensed under the Apache License, Version 2.0.