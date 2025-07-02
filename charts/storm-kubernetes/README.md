# Apache Storm Helm Chart for Kubernetes

This Helm chart deploys Apache Storm on Kubernetes with support for:
- CRD-based topology management via kubectl
- Per-topology worker pools with independent scaling
- High Availability Nimbus
- Embedded or external Zookeeper
- Prometheus monitoring
- Zero-downtime upgrades

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

This chart uses Custom Resource Definitions (CRDs) to manage Storm topologies. After installing the chart, you can deploy topologies using `kubectl`:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: my-topology
  namespace: default
spec:
  clusterRef: my-storm
  topology:
    name: wordcount
    jar:
      url: "https://example.com/my-topology.jar"
    mainClass: "com.example.MyTopology"
  workers:
    replicas: 2
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
    autoscaling:
      enabled: true
      maxReplicas: 10
```

Apply the topology:

```bash
kubectl apply -f my-topology.yaml
```

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
| `image.tag` | Storm image tag | `2.6.0` |
| `image.pullPolicy` | Storm image pull policy | `IfNotPresent` |

### Nimbus parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nimbus.replicaCount` | Number of Nimbus replicas (HA mode) | `1` |
| `nimbus.resources.requests` | Nimbus container resource requests | `{cpu: 500m, memory: 1Gi}` |
| `nimbus.resources.limits` | Nimbus container resource limits | `{cpu: 2000m, memory: 2Gi}` |
| `nimbus.persistence.enabled` | Enable persistence for Nimbus | `true` |
| `nimbus.persistence.size` | Nimbus persistent volume size | `10Gi` |
| `nimbus.persistence.storageClass` | Nimbus persistent volume storage class | `""` |

### Supervisor parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `supervisor.replicaCount` | Number of Supervisor replicas | `3` |
| `supervisor.deploymentMode` | Deployment mode (`deployment` or `daemonset`) | `deployment` |
| `supervisor.slotsPerSupervisor` | Number of worker slots per supervisor | `4` |
| `supervisor.resources.requests` | Supervisor container resource requests | `{cpu: 1000m, memory: 2Gi}` |
| `supervisor.resources.limits` | Supervisor container resource limits | `{cpu: 2000m, memory: 4Gi}` |
| `supervisor.autoscaling.enabled` | Enable HPA for supervisors | `false` |
| `supervisor.autoscaling.minReplicas` | Minimum supervisor replicas | `3` |
| `supervisor.autoscaling.maxReplicas` | Maximum supervisor replicas | `10` |

### UI parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.enabled` | Enable Storm UI | `true` |
| `ui.replicaCount` | Number of UI replicas | `1` |
| `ui.service.type` | UI service type | `ClusterIP` |
| `ui.ingress.enabled` | Enable ingress for UI | `false` |
| `ui.ingress.hostname` | Hostname for UI ingress | `storm.local` |
| `ui.auth.enabled` | Enable UI authentication | `false` |
| `ui.auth.type` | Authentication type (`simple` or `oauth`) | `simple` |

### Controller parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.enabled` | Enable Storm Controller for CRD management | `true` |
| `controller.replicaCount` | Number of controller replicas | `1` |
| `controller.image.repository` | Controller image repository | `apache/storm-controller` |
| `controller.image.tag` | Controller image tag | `0.1.0` |

### Zookeeper parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `zookeeper.enabled` | Deploy Zookeeper | `true` |
| `zookeeper.replicaCount` | Number of Zookeeper replicas | `3` |
| `zookeeper.persistence.enabled` | Enable persistence for Zookeeper | `true` |
| `zookeeper.persistence.size` | Zookeeper persistent volume size | `8Gi` |
| `externalZookeeper.servers` | External Zookeeper servers (if not using embedded) | `[]` |

### Monitoring parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics export | `true` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor resource | `true` |
| `metrics.serviceMonitor.interval` | Scrape interval | `30s` |
| `metrics.prometheusRule.enabled` | Create PrometheusRule resource | `false` |

### Storm configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `stormConf` | Additional Storm configuration | `{}` |

## Persistence

The chart mounts a Persistent Volume for Nimbus to store topology jars and metadata. You can configure the size and storage class of the volume.

## Monitoring

The chart includes support for Prometheus monitoring:

1. Metrics are exposed on `/metrics` endpoint
2. ServiceMonitor CRD for automatic Prometheus discovery
3. PrometheusRule CRD for alerting rules

## Upgrading

### To 1.0.0

This version introduces breaking changes:
- CRDs are now used for topology management
- Controller component added for CRD reconciliation
- Per-topology worker pools instead of shared workers

## License

Copyright 2024 The Apache Software Foundation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.