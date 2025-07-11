# Storm CRD Cluster Helm Chart

This Helm chart creates a StormCluster Custom Resource (CR) that is managed by the Storm Operator. It provides a declarative way to deploy Apache Storm clusters on Kubernetes.

## Overview

This chart creates:
- A StormCluster CR that defines the desired Storm cluster configuration
- The Storm Operator then reconciles this CR to create:
  - Nimbus StatefulSet(s)
  - Supervisor Deployment(s) or DaemonSet
  - Storm UI Deployment (optional)
  - ConfigMaps for Storm configuration
  - Services for cluster communication

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- **Storm Operator must be installed** (see storm-operator chart)
- Zookeeper (either from storm-operator or external)

## Installation

### Quick Start

Install a basic Storm cluster:

```bash
helm install my-storm-cluster ./charts/storm-crd-cluster \
  --namespace storm-system
```

### Using Example Values Files

Install with minimal configuration:
```bash
helm install my-storm-cluster ./charts/storm-crd-cluster \
  -f ./charts/storm-crd-cluster/values-minimal.yaml \
  --namespace storm-system
```

Install with production configuration:
```bash
helm install my-storm-cluster ./charts/storm-crd-cluster \
  -f ./charts/storm-crd-cluster/values-production.yaml \
  --namespace storm-system
```

Install for local development:
```bash
helm install my-storm-cluster ./charts/storm-crd-cluster \
  -f ./charts/storm-crd-cluster/storm-crd-cluster-local-values.yaml \
  --namespace storm-system
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `storm.image.repository` | Storm image repository | `storm` |
| `storm.image.tag` | Storm image tag | `2.8.1` |
| `storm.config` | Storm configuration (storm.yaml content) | See values.yaml |
| `nimbus.replicas` | Number of Nimbus instances | `1` |
| `nimbus.resources` | Nimbus resource requests/limits | See values.yaml |
| `supervisor.replicas` | Number of Supervisor instances | `1` |
| `supervisor.slotsPerSupervisor` | Worker slots per supervisor | `1` |
| `supervisor.resources` | Supervisor resource requests/limits | See values.yaml |
| `ui.enabled` | Enable Storm UI | `true` |
| `ui.resources` | UI resource requests/limits | See values.yaml |

### Zookeeper Configuration

#### Using Default Zookeeper from Operator
```yaml
zookeeper:
  external:
    enabled: false
  default:
    enabled: true
    operatorNamespace: "storm-system"
    serviceName: "storm-operator-zookeeper-headless"
```

#### Using External Zookeeper
```yaml
zookeeper:
  external:
    enabled: true
    servers:
      - "zookeeper-0.zookeeper-headless.zookeeper.svc.cluster.local"
      - "zookeeper-1.zookeeper-headless.zookeeper.svc.cluster.local"
      - "zookeeper-2.zookeeper-headless.zookeeper.svc.cluster.local"
    root: "/storm/production"
  default:
    enabled: false
```

### Storm Configuration

Customize Storm settings through the `storm.config` parameter:

```yaml
storm:
  config:
    # Logging
    storm.log.level: "INFO"
    storm.log.dir: "/logs"
    
    # Nimbus settings
    nimbus.task.timeout.secs: "30"
    nimbus.supervisor.timeout.secs: "60"
    
    # Supervisor settings
    supervisor.worker.timeout.secs: "30"
    supervisor.slots.ports: [6700, 6701, 6702, 6703]
    
    # Worker settings
    worker.childopts: "-Xmx1024m -XX:+UseG1GC"
    worker.heap.memory.mb: "1024"
    
    # Topology defaults
    topology.message.timeout.secs: "30"
    topology.max.task.parallelism: "128"
```

### Resource Configuration

#### Nimbus Resources
```yaml
nimbus:
  replicas: 2  # HA configuration
  resources:
    requests:
      cpu: 1000m
      memory: 2Gi
    limits:
      cpu: 2000m
      memory: 4Gi
```

#### Supervisor Resources
```yaml
supervisor:
  replicas: 3
  slotsPerSupervisor: 4  # 4 worker slots per supervisor
  resources:
    requests:
      cpu: 2000m
      memory: 4Gi
    limits:
      cpu: 4000m
      memory: 8Gi
```

### Persistence

Enable persistent storage:
```yaml
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 50Gi
```

### Node Affinity and Tolerations

Deploy on specific nodes:
```yaml
nimbus:
  nodeSelector:
    node-role.kubernetes.io/storm: "true"
  tolerations:
    - key: "storm"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"

supervisor:
  nodeSelector:
    node-role.kubernetes.io/storm: "true"
  tolerations:
    - key: "storm"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"
```

## Usage

### Verify Cluster Creation

Check the StormCluster resource:
```bash
kubectl get stormcluster -n storm-system
kubectl describe stormcluster my-storm-cluster -n storm-system
```

Check created pods:
```bash
kubectl get pods -n storm-system -l storm.apache.org/cluster=my-storm-cluster
```

### Access Storm UI

If UI is enabled:
```bash
kubectl port-forward -n storm-system svc/my-storm-cluster-ui 8080:8080
```

Then access http://localhost:8080

### Deploy a Topology

After the cluster is running, deploy a topology:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount
  namespace: storm-system
spec:
  clusterName: my-storm-cluster  # Must match your cluster name
  version: "1.0.0"
  jar:
    url: "https://example.com/wordcount-topology.jar"
  mainClass: "org.apache.storm.examples.WordCountTopology"
  args:
    - "wordcount"
```

## Advanced Configuration

### High Availability Nimbus

For production, use multiple Nimbus instances:
```yaml
nimbus:
  replicas: 2
  
storm:
  config:
    nimbus.seeds: ["nimbus-0", "nimbus-1"]
```

### Custom Labels and Annotations

Add custom metadata:
```yaml
commonLabels:
  environment: "production"
  team: "data-platform"

commonAnnotations:
  "cost-center": "engineering"
  "backup.velero.io/backup-volumes": "nimbus-data,storm-logs"
```

### Monitoring

Enable metrics collection:
```yaml
monitoring:
  enabled: true
  port: 8080
```

## Troubleshooting

### Check Cluster Status

```bash
# Get cluster status
kubectl get stormcluster -n storm-system

# Check detailed status
kubectl describe stormcluster my-storm-cluster -n storm-system

# Check operator logs
kubectl logs -n storm-system -l app.kubernetes.io/name=storm-operator
```

### Common Issues

1. **Cluster Stuck in Creating State**
   - Check operator logs for errors
   - Verify Zookeeper connectivity
   - Check resource quotas and limits

2. **Pods Not Starting**
   - Check pod events: `kubectl describe pod <pod-name> -n storm-system`
   - Verify volume mounts and ConfigMaps
   - Check node resources and scheduling constraints

3. **Configuration Issues**
   - Verify storm.yaml syntax in ConfigMap
   - Check for string vs integer type issues in config
   - Ensure all required Storm settings are present

### Debug Commands

```bash
# Check generated ConfigMap
kubectl get configmap -n storm-system -l storm.apache.org/cluster=my-storm-cluster
kubectl describe configmap my-storm-cluster-config -n storm-system

# Check services
kubectl get svc -n storm-system -l storm.apache.org/cluster=my-storm-cluster

# Execute into a pod
kubectl exec -it -n storm-system my-storm-cluster-nimbus-0 -- bash
```

## Uninstallation

Remove the Storm cluster:
```bash
helm uninstall my-storm-cluster -n storm-system
```

This will delete the StormCluster CR, and the operator will clean up all associated resources.

## Examples

### Minimal Development Cluster
```yaml
storm:
  config:
    storm.log.level: "DEBUG"
nimbus:
  replicas: 1
supervisor:
  replicas: 1
  slotsPerSupervisor: 1
ui:
  enabled: true
persistence:
  enabled: false
```

### Production HA Cluster
```yaml
nimbus:
  replicas: 2
supervisor:
  replicas: 5
  slotsPerSupervisor: 4
persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 100Gi
monitoring:
  enabled: true
```

## Version Compatibility

| Chart Version | Storm Version | Operator Version |
|--------------|---------------|------------------|
| 0.1.x | 2.8.x | 0.1.x |

## License

This project is licensed under the Apache License 2.0.