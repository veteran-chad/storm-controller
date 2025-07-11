# Enhanced StormCluster Controller

## Overview

The enhanced StormCluster controller adds full cluster provisioning capabilities to the Storm Kubernetes operator. Unlike the original read-only implementation, this controller can now create and manage complete Storm clusters including Nimbus, Supervisors, UI, and all associated resources.

## Features

### Cluster Provisioning
- **Nimbus StatefulSet**: Deploys Nimbus nodes with persistent storage for HA configurations
- **Supervisor Deployment/DaemonSet**: Flexible deployment modes for worker nodes
- **Storm UI**: Optional web interface with ingress support
- **Configuration Management**: Automatic storm.yaml generation via ConfigMap
- **Service Discovery**: Headless services for internal communication

### High Availability
- Multi-Nimbus support with leader election
- Persistent storage for Nimbus state
- Anti-affinity rules for spreading components

### Resource Management
- Configurable resource requests and limits
- Node selectors and tolerations
- Pod affinity/anti-affinity rules

### Monitoring & Observability
- Prometheus metrics exposure
- ServiceMonitor creation for Prometheus Operator
- Detailed status reporting

### Security
- Image pull secrets support
- Basic authentication for Storm UI
- TLS/Ingress configuration

## Architecture

```
StormCluster CR
    |
    +-- ConfigMap (storm.yaml)
    |
    +-- Nimbus StatefulSet
    |   +-- Persistent Volume Claims
    |   +-- Headless Service
    |
    +-- Supervisor Deployment/DaemonSet
    |
    +-- UI Deployment
    |   +-- Service (ClusterIP/NodePort/LoadBalancer)
    |   +-- Ingress (optional)
    |
    +-- Metrics endpoints
```

## Usage

### Basic Development Cluster

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormCluster
metadata:
  name: storm-dev
spec:
  nimbus:
    replicas: 1
  supervisor:
    replicas: 2
    workerSlots: 2
  ui:
    enabled: true
    service:
      type: NodePort
  zookeeper:
    enabled: true
    externalServers:
      - "zookeeper:2181"
```

### Production HA Cluster

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormCluster
metadata:
  name: storm-prod
spec:
  nimbus:
    replicas: 3
    persistence:
      enabled: true
      size: "10Gi"
      storageClass: "fast-ssd"
    resources:
      requests:
        memory: "2Gi"
        cpu: "1"
    tolerations:
      - key: "storm-nimbus"
        operator: "Equal"
        value: "true"
        effect: "NoSchedule"
  
  supervisor:
    replicas: 10
    deploymentMode: "deployment"
    workerSlots: 4
    resources:
      requests:
        memory: "4Gi"
        cpu: "2"
    nodeSelector:
      storm-role: worker
  
  ui:
    enabled: true
    replicas: 2
    service:
      type: LoadBalancer
    ingress:
      enabled: true
      hostname: "storm.example.com"
      tls: true
  
  zookeeper:
    enabled: true
    externalServers:
      - "zk-0.zk-headless:2181"
      - "zk-1.zk-headless:2181"
      - "zk-2.zk-headless:2181"
```

## Configuration Options

### Image Configuration
- `image.repository`: Docker image repository (default: apache/storm)
- `image.tag`: Storm version tag (default: 2.6.0)
- `image.pullPolicy`: Image pull policy
- `image.pullSecrets`: List of image pull secrets

### Nimbus Configuration
- `nimbus.replicas`: Number of Nimbus nodes (1-5)
- `nimbus.resources`: CPU/memory requests and limits
- `nimbus.persistence`: PVC configuration for Nimbus data
- `nimbus.thrift`: Thrift server configuration
- `nimbus.nodeSelector`: Node selection constraints
- `nimbus.tolerations`: Pod tolerations
- `nimbus.affinity`: Pod affinity rules

### Supervisor Configuration
- `supervisor.replicas`: Number of supervisor nodes
- `supervisor.deploymentMode`: "deployment" or "daemonset"
- `supervisor.workerSlots`: Worker slots per supervisor
- `supervisor.resources`: CPU/memory for supervisors
- `supervisor.nodeSelector`: Node selection constraints
- `supervisor.tolerations`: Pod tolerations

### UI Configuration
- `ui.enabled`: Enable Storm UI
- `ui.replicas`: Number of UI instances
- `ui.service`: Service configuration
- `ui.ingress`: Ingress configuration
- `ui.auth`: Authentication settings

### Zookeeper Configuration
- `zookeeper.enabled`: Use external Zookeeper
- `zookeeper.externalServers`: List of Zookeeper servers
- `zookeeper.chrootPath`: Zookeeper chroot path

## Status Fields

The controller updates the following status fields:

```yaml
status:
  phase: Running  # Pending, Creating, Running, Failed, Degraded
  nimbusReady: 3
  supervisorReady: 10
  uiReady: 2
  nimbusLeader: storm-prod-nimbus-0
  nimbusNodes:
    - storm-prod-nimbus-0
    - storm-prod-nimbus-1
    - storm-prod-nimbus-2
  totalSlots: 40
  usedSlots: 25
  freeSlots: 15
  topologyCount: 5
  endpoints:
    nimbus: storm-prod-nimbus.default.svc.cluster.local:6627
    ui: storm-prod-ui.default.svc.cluster.local:8080
    restApi: http://storm-prod-ui.default.svc.cluster.local:8080/api/v1
  conditions:
    - type: Available
      status: "True"
      reason: ClusterHealthy
      message: Storm cluster is healthy and available
```

## Metrics

The controller exposes the following Prometheus metrics:

- `storm_cluster_info`: Cluster information (labels: cluster, namespace, version)
- `storm_cluster_supervisors`: Number of supervisor nodes
- `storm_cluster_slots{state="total|used|free"}`: Worker slot metrics

## Cross-Resource Watching

The enhanced controller watches for changes in:
- StormTopology resources that reference this cluster
- Owned resources (StatefulSets, Deployments, Services, ConfigMaps)

This enables automatic reconciliation when:
- A topology is submitted to the cluster
- Cluster resources are modified outside the controller
- Related resources change state

## Migration from Existing Clusters

To migrate from an existing Storm cluster:

1. Create a StormCluster CR matching your existing configuration
2. The controller will adopt existing resources if they have matching labels
3. Gradually update the CR to manage the cluster through the operator

## Limitations

- Zookeeper deployment is not managed (use external Zookeeper)
- No automatic Storm version upgrades
- No backup/restore functionality
- Authentication is basic (username/password only)

## Future Enhancements

- Integrated Zookeeper deployment
- Advanced security (Kerberos, OAuth)
- Automated backups
- Rolling upgrades
- Multi-region support