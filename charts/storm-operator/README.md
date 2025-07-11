# Storm Operator Helm Chart

This Helm chart deploys the Storm Operator for Kubernetes, which manages Apache Storm clusters through Custom Resource Definitions (CRDs).

## Overview

The Storm Operator provides:
- Kubernetes-native management of Apache Storm clusters
- Automated topology deployment and lifecycle management
- Version-based topology updates with zero downtime
- Container-based JAR deployment support
- Integration with GitOps workflows

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- kubectl configured to access your cluster

## Installation

### Quick Start

Install the operator with default settings:

```bash
helm install storm-operator ./charts/storm-operator \
  --namespace storm-system \
  --create-namespace
```

### Using Example Values Files

Install with minimal configuration:
```bash
helm install storm-operator ./charts/storm-operator \
  -f ./charts/storm-operator/values-minimal.yaml \
  --namespace storm-system \
  --create-namespace
```

Install with production configuration:
```bash
helm install storm-operator ./charts/storm-operator \
  -f ./charts/storm-operator/values-production.yaml \
  --namespace storm-system \
  --create-namespace
```

Install for local development:
```bash
helm install storm-operator ./charts/storm-operator \
  -f ./charts/storm-operator/storm-local-values.yaml \
  --namespace storm-system \
  --create-namespace
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.replicaCount` | Number of operator replicas | `1` |
| `operator.image.repository` | Operator image repository | `storm-operator` |
| `operator.image.tag` | Operator image tag | `latest` |
| `operator.resources` | CPU/Memory resource requests/limits | See values.yaml |
| `zookeeper.enabled` | Deploy Zookeeper with the operator | `true` |
| `zookeeper.replicaCount` | Number of Zookeeper replicas | `1` |
| `crd.install` | Install Storm CRDs | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `rbac.create` | Create RBAC resources | `true` |

### Zookeeper Configuration

The operator can deploy its own Zookeeper instance or use an external one:

#### Using Built-in Zookeeper
```yaml
zookeeper:
  enabled: true
  replicaCount: 3
  persistence:
    enabled: true
    size: 10Gi
```

#### Using External Zookeeper
```yaml
zookeeper:
  enabled: false

# Configure in your StormCluster resources:
# zookeeper:
#   servers:
#     - "external-zookeeper-1:2181"
#     - "external-zookeeper-2:2181"
```

### Security Configuration

#### Pod Security Context
```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault

containerSecurityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

#### Network Policies
```yaml
networkPolicy:
  enabled: true
  allowExternal: false
```

### Monitoring

Enable Prometheus monitoring:
```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
```

### High Availability

For production deployments:
```yaml
operator:
  replicaCount: 3
  podAntiAffinityPreset: hard
  
pdb:
  create: true
  minAvailable: 2
```

## Usage

### Deploy a Storm Cluster

After installing the operator, create a Storm cluster:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormCluster
metadata:
  name: my-cluster
  namespace: storm-system
spec:
  managementMode: create
  image:
    repository: storm
    tag: "2.8.1"
  nimbus:
    replicas: 1
  supervisor:
    replicas: 3
    slotsPerSupervisor: 4
  ui:
    enabled: true
  zookeeper:
    servers:
      - "storm-operator-zookeeper-headless.storm-system.svc.cluster.local"
```

Apply the cluster:
```bash
kubectl apply -f my-storm-cluster.yaml
```

### Deploy a Topology

Deploy a Storm topology:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount
  namespace: storm-system
spec:
  clusterName: my-cluster
  version: "1.0.0"
  jar:
    url: "https://example.com/wordcount-topology.jar"
  mainClass: "org.apache.storm.examples.WordCountTopology"
  args:
    - "wordcount"
```

## Advanced Configuration

### Resource Quotas

Set resource quotas for the namespace:
```yaml
resourceQuota:
  enabled: true
  hard:
    requests.cpu: "100"
    requests.memory: "200Gi"
    persistentvolumeclaims: "50"
```

### Custom Storm Configuration

Override default Storm configuration:
```yaml
stormDefaults:
  config:
    nimbus.task.timeout.secs: 30
    supervisor.worker.timeout.secs: 30
    topology.message.timeout.secs: 30
```

### Tolerations and Node Affinity

Deploy on specific nodes:
```yaml
operator:
  nodeSelector:
    node-role.kubernetes.io/storm: "true"
  
  tolerations:
    - key: "storm-operator"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"
```

## Troubleshooting

### Check Operator Status
```bash
kubectl get pods -n storm-system -l app.kubernetes.io/name=storm-operator
kubectl logs -n storm-system -l app.kubernetes.io/name=storm-operator
```

### Verify CRDs
```bash
kubectl get crd | grep storm
```

### Common Issues

1. **Operator CrashLoopBackOff**
   - Check logs: `kubectl logs -n storm-system <operator-pod>`
   - Verify RBAC permissions
   - Check health probe configuration

2. **ConfigMap Not Found**
   - Ensure the operator ConfigMap is created
   - Check the ConfigMap name matches the deployment

3. **Zookeeper Connection Issues**
   - Verify Zookeeper is running
   - Check service DNS resolution
   - Ensure correct Zookeeper address format (without port in server list)

## Uninstallation

Remove the operator:
```bash
helm uninstall storm-operator -n storm-system
```

Clean up CRDs (if desired):
```bash
kubectl delete crd stormclusters.storm.apache.org
kubectl delete crd stormtopologies.storm.apache.org
kubectl delete crd stormworkerpools.storm.apache.org
```

## Version Compatibility

| Operator Version | Storm Version | Kubernetes Version |
|-----------------|---------------|-------------------|
| 0.1.x | 2.8.x | 1.19+ |

## Contributing

Please refer to the main project repository for contribution guidelines.

## License

This project is licensed under the Apache License 2.0.