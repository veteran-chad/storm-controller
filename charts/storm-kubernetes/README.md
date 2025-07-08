# Apache Storm Helm Chart for Kubernetes

This Helm chart deploys Apache Storm on Kubernetes with support for:
- High Availability Nimbus
- Configurable Supervisor worker slots
- Embedded or external Zookeeper
- Storm UI with optional ingress
- Prometheus monitoring
- Persistent storage for logs and data

## Prerequisites

- Kubernetes 1.23+
- Helm 3.8+
- PV provisioner support in the underlying infrastructure (for persistence)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) (optional, for monitoring)

## Installation

### Add the Helm repository

```bash
helm repo add apache https://apache.github.io/storm
helm repo update
```

### Install the chart

```bash
helm install my-storm apache/storm-kubernetes
```

### Install with custom values

```bash
helm install my-storm apache/storm-kubernetes \
  --set nimbus.replicaCount=3 \
  --set supervisor.replicaCount=5 \
  --set ui.ingress.enabled=true \
  --set ui.ingress.hostname=storm.example.com
```

## Deploying Topologies

After installing the chart, you can deploy topologies using the Storm CLI:

1. Port-forward to the Nimbus service:
```bash
kubectl port-forward svc/my-storm-nimbus 6627:6627
```

2. Submit a topology:
```bash
storm jar my-topology.jar com.example.MyTopology my-topology
```

For automated topology management, consider using the [Storm Operator](../storm-operator/README.md).

## Configuration

The following table lists the main configurable parameters of the Storm chart and their default values.

### Global parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imageRegistry` | Global Docker image registry | `""` |
| `global.imagePullSecrets` | Global Docker registry secret names | `[]` |
| `global.storageClass` | Global StorageClass for PVCs | `""` |

### Storm image parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Storm image registry | `docker.io` |
| `image.repository` | Storm image repository | `apache/storm` |
| `image.tag` | Storm image tag | `2.8.1` |
| `image.pullPolicy` | Storm image pull policy | `IfNotPresent` |

### Nimbus parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nimbus.replicaCount` | Number of Nimbus replicas | `1` |
| `nimbus.resources.limits` | Resource limits | `{cpu: 1000m, memory: 2Gi}` |
| `nimbus.resources.requests` | Resource requests | `{cpu: 250m, memory: 512Mi}` |
| `nimbus.persistence.enabled` | Enable persistence | `true` |
| `nimbus.persistence.size` | PVC size | `8Gi` |

### Supervisor parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `supervisor.replicaCount` | Number of Supervisor nodes | `3` |
| `supervisor.slotsPerSupervisor` | Worker slots per supervisor | `4` |
| `supervisor.resources.limits` | Resource limits | `{cpu: 2000m, memory: 4Gi}` |
| `supervisor.resources.requests` | Resource requests | `{cpu: 1000m, memory: 2Gi}` |
| `supervisor.deploymentMode` | `deployment` or `daemonset` | `deployment` |

### UI parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.enabled` | Enable Storm UI | `true` |
| `ui.replicaCount` | Number of UI replicas | `1` |
| `ui.service.type` | Service type | `ClusterIP` |
| `ui.ingress.enabled` | Enable ingress | `false` |
| `ui.ingress.hostname` | Ingress hostname | `storm.local` |

### Zookeeper parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `zookeeper.enabled` | Deploy Zookeeper | `true` |
| `zookeeper.replicaCount` | Number of Zookeeper nodes | `3` |
| `externalZookeeper.servers` | External Zookeeper servers | `[]` |

### Storm configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `storm.config` | Storm configuration (storm.yaml) | See values.yaml |

## Persistence

The chart supports persistent volumes for Nimbus and Zookeeper data. To disable persistence:

```bash
helm install my-storm apache/storm-kubernetes \
  --set nimbus.persistence.enabled=false \
  --set zookeeper.persistence.enabled=false
```

## Monitoring

The chart includes ServiceMonitor resources for Prometheus Operator integration:

```bash
helm install my-storm apache/storm-kubernetes \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

## High Availability

For production deployments, enable HA mode:

```bash
helm install my-storm apache/storm-kubernetes \
  --set nimbus.replicaCount=3 \
  --set zookeeper.replicaCount=3 \
  --set ui.replicaCount=2
```

## External Zookeeper

To use an external Zookeeper cluster:

```bash
helm install my-storm apache/storm-kubernetes \
  --set zookeeper.enabled=false \
  --set externalZookeeper.servers='{zk1:2181,zk2:2181,zk3:2181}'
```

## Uninstalling

To uninstall/delete the deployment:

```bash
helm uninstall my-storm
```

## Upgrading

To upgrade the Storm deployment:

```bash
helm upgrade my-storm apache/storm-kubernetes \
  --set image.tag=2.8.1
```

## License

Copyright 2024 The Apache Software Foundation

Licensed under the Apache License, Version 2.0.