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
| `supervisor.replicaCount` | Number of Supervisor nodes | `3` |
| `supervisor.slotsPerSupervisor` | Worker slots per supervisor | `4` |
| `supervisor.autoMemory.enabled` | Enable automatic memory calculation | `false` |
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
  serviceMonitor:
    enabled: true
  prometheusRule:
    enabled: true
```

#### Autoscaling
```yaml
# HPA for supervisors
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
```

### Memory Configuration

The chart supports automatic memory calculation for optimal performance:

```yaml
supervisor:
  autoMemory:
    enabled: true
    containerMemoryFactor: 0.8  # 80% of container memory for JVM
    jvmMemoryFactor: 0.75      # 75% of JVM memory for workers
  
  resources:
    requests:
      memory: 4Gi  # Results in ~2.4GB per worker
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

The chart includes pre-configured Prometheus alerts:

- **StormClusterDown**: No Storm metrics for 5 minutes
- **StormHighSlotUsage**: >90% slot utilization
- **StormNimbusUnavailable**: Nimbus unreachable
- **StormSupervisorDown**: Supervisor failures

Access metrics at: `http://<metrics-exporter>:9102/metrics`

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