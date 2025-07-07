# Enhanced StormWorkerPool Controller

## Overview

The enhanced StormWorkerPool controller provides advanced worker management capabilities for Storm topologies running in Kubernetes. It enables fine-grained control over worker resources, autoscaling, and pod customization.

## Features

### Core Functionality
- **Dynamic Worker Management**: Create and manage dedicated worker pools for specific topologies
- **Resource Isolation**: Separate worker resources from supervisor nodes
- **Pod Template Customization**: Full control over worker pod specifications
- **Multi-Container Support**: Add sidecar containers for logging, monitoring, etc.

### Autoscaling
- **Horizontal Pod Autoscaling**: Automatic scaling based on metrics
- **CPU and Memory Metrics**: Built-in support for resource-based scaling
- **Custom Metrics**: Scale based on Storm-specific metrics (pending tuples, latency, etc.)
- **Configurable Scaling Behavior**: Control scale-up/down rates and stabilization

### Advanced Configuration
- **JVM Tuning**: Per-pool JVM options and heap settings
- **Port Management**: Configurable worker port ranges
- **Volume Support**: Mount persistent volumes, ConfigMaps, and Secrets
- **Node Selection**: Affinity, anti-affinity, tolerations, and node selectors
- **GPU Support**: Allocate GPU resources for ML workloads

## Architecture

```
StormTopology
    |
    +-- StormWorkerPool(s)
        |
        +-- Deployment
        |   +-- ReplicaSet
        |       +-- Worker Pods
        |
        +-- HorizontalPodAutoscaler (optional)
        |
        +-- Service (headless for discovery)
```

## Usage

### Basic Worker Pool

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormWorkerPool
metadata:
  name: my-workers
spec:
  topologyRef: my-topology
  replicas: 3
  template:
    spec:
      containers:
        - name: worker
          resources:
            requests:
              memory: "1Gi"
              cpu: "1"
```

### Worker Pool with Autoscaling

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormWorkerPool
metadata:
  name: autoscale-workers
spec:
  topologyRef: streaming-topology
  replicas: 2  # Initial replicas
  
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
    customMetrics:
      - name: storm_pending_messages
        type: pods
        targetAverageValue: "1000"
```

### GPU-Enabled Worker Pool

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormWorkerPool
metadata:
  name: gpu-workers
spec:
  topologyRef: ml-topology
  replicas: 4
  
  template:
    spec:
      containers:
        - name: worker
          resources:
            requests:
              nvidia.com/gpu: "1"
            limits:
              nvidia.com/gpu: "1"
      nodeSelector:
        accelerator: nvidia-tesla-v100
```

## Configuration Options

### Basic Settings
- `topologyRef`: Name of the StormTopology this pool serves (required)
- `clusterRef`: Override cluster reference (defaults to topology's cluster)
- `replicas`: Number of worker instances (ignored when autoscaling is enabled)

### Image Configuration
- `image.repository`: Custom worker image repository
- `image.tag`: Custom worker image tag
- `image.pullPolicy`: Image pull policy
- `image.pullSecrets`: Image pull secrets

### Worker Configuration
- `workerConfig`: Storm-specific worker configuration
- `jvmOpts`: JVM options for worker processes
- `ports.start`: Starting port number (default: 6700)
- `ports.count`: Number of ports per worker (default: 4)

### Pod Template
- `template.metadata.labels`: Additional labels for worker pods
- `template.metadata.annotations`: Annotations for worker pods
- `template.spec`: Full pod spec customization including:
  - Container resources and environment
  - Volumes and volume mounts
  - Affinity and anti-affinity rules
  - Tolerations and node selectors
  - Security context
  - Service account

### Autoscaling
- `autoscaling.enabled`: Enable horizontal pod autoscaling
- `autoscaling.minReplicas`: Minimum number of replicas
- `autoscaling.maxReplicas`: Maximum number of replicas
- `autoscaling.targetCPUUtilizationPercentage`: Target CPU utilization
- `autoscaling.targetMemoryUtilizationPercentage`: Target memory utilization
- `autoscaling.customMetrics`: Custom metrics for scaling decisions

## Status Fields

```yaml
status:
  phase: Running
  replicas: 5
  readyReplicas: 5
  availableReplicas: 5
  unavailableReplicas: 0
  updatedReplicas: 5
  deploymentName: my-workers-deployment
  hpaName: my-workers-hpa
  message: "All 5 worker(s) are ready"
  conditions:
    - type: Ready
      status: "True"
      reason: WorkersReady
      message: "All 5 worker(s) are ready"
    - type: Autoscaling
      status: "True"
      reason: HPAActive
      message: "HPA is active (current: 5, min: 2, max: 20)"
```

## Metrics

The controller exposes Prometheus metrics:

- `storm_worker_pool_replicas{pool,namespace,topology,state}`: Number of replicas by state (desired, ready, available)
- `storm_worker_pool_info{pool,namespace,topology,phase}`: Worker pool information

## Use Cases

### 1. Topology-Specific Resource Allocation
Different topologies may have different resource requirements. Worker pools allow you to:
- Allocate high-memory workers for data processing topologies
- Use CPU-optimized workers for computation-heavy topologies
- Assign GPU workers for machine learning topologies

### 2. Multi-Tenancy
Isolate workloads by:
- Using different node pools for different customers
- Applying resource quotas per worker pool
- Implementing network policies between pools

### 3. Cost Optimization
- Scale down development/testing topologies during off-hours
- Use spot instances for fault-tolerant batch processing
- Right-size workers based on actual workload

### 4. Performance Tuning
- Experiment with different JVM settings per topology
- Adjust worker counts based on throughput requirements
- Fine-tune resource allocations without affecting other topologies

## Best Practices

1. **Resource Requests and Limits**: Always set appropriate resource requests and limits to ensure proper scheduling and prevent resource starvation.

2. **Autoscaling Metrics**: Choose metrics that accurately reflect your topology's load. CPU/memory may not always be the best indicators.

3. **Pod Disruption Budgets**: Create PDBs to ensure availability during cluster maintenance.

4. **Monitoring**: Use the exposed metrics and Storm UI to monitor worker pool performance.

5. **Gradual Rollouts**: Use the deployment's rolling update strategy to safely update worker configurations.

## Troubleshooting

### Workers Not Starting
- Check topology status: Ensure the referenced topology is running
- Verify cluster status: The Storm cluster must be healthy
- Review resource availability: Ensure nodes have sufficient resources
- Check pod events: `kubectl describe pod <worker-pod>`

### Autoscaling Not Working
- Verify metrics server is installed
- Check HPA status: `kubectl describe hpa <workerpool-name>-hpa`
- Ensure resource requests are set (required for percentage-based scaling)
- Validate custom metrics are available

### Performance Issues
- Review JVM settings and heap allocation
- Check for resource throttling
- Verify network connectivity between workers and Nimbus
- Analyze Storm UI for bottlenecks

## Migration Guide

To migrate from default Storm supervisors to worker pools:

1. Create worker pool with similar resource configuration
2. Test with a small replica count
3. Gradually increase replicas while monitoring performance
4. Optionally reduce supervisor count to save resources

## Future Enhancements

- Vertical Pod Autoscaling (VPA) integration
- Topology-aware scheduling
- Automatic JVM tuning based on workload
- Cost-based autoscaling
- Multi-region worker pools