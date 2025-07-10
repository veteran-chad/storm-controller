# Storm Memory Management Recommendation

## Executive Summary

**Recommended Approach: Hybrid Auto/Manual Configuration**

Use an automatic memory calculation system with manual override capability. This provides safe defaults for most users while allowing experts to fine-tune when needed.

## Key Benefits

1. **Prevents Common Mistakes**: Auto-calculates container resources based on worker requirements
2. **Simple Scaling**: Just change `memoryPerWorker` and `slotsPerSupervisor`
3. **Resource Efficient**: Ensures proper memory allocation without waste
4. **Production Ready**: Handles JVM overhead and Storm's resource scheduler

## How It Works

```
┌─────────────────────────────────────────────────┐
│          Supervisor Pod (Container)             │
│  Memory: 5Gi (calculated with 25% overhead)     │
│                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────┐ │
│  │  Worker 1   │ │  Worker 2   │ │ Worker 3  │ │
│  │ Heap: 820MB │ │ Heap: 820MB │ │Heap: 820MB│ │
│  │ Total: 1Gi  │ │ Total: 1Gi  │ │Total: 1Gi │ │
│  └─────────────┘ └─────────────┘ └───────────┘ │
│                                                 │
│  ┌─────────────┐     JVM Overhead:             │
│  │  Worker 4   │     - Metaspace               │
│  │ Heap: 820MB │     - Direct Memory           │
│  │ Total: 1Gi  │     - Thread Stacks           │
│  └─────────────┘     - GC Overhead             │
└─────────────────────────────────────────────────┘
```

## Configuration Example

```yaml
supervisor:
  # Simple configuration - just set these:
  replicaCount: 3
  slotsPerSupervisor: 4
  
  memoryConfig:
    mode: "auto"
    memoryPerWorker: "1Gi"      # Primary tuning parameter
    memoryOverheadPercent: 25    # JVM overhead (25% is safe default)
    cpuPerWorker: "1"            # CPU per worker
```

This automatically configures:
- Container memory: 5Gi (4 workers × 1Gi + 25% overhead)
- Worker heap: 820MB (80% of 1Gi)
- Supervisor capacity: 4096MB
- Storm configuration values

## Scaling Patterns

### Horizontal Scaling (More Workers)
```yaml
# Add more supervisor pods
supervisor:
  replicaCount: 5  # Was 3, now 5
  memoryConfig:
    memoryPerWorker: "1Gi"
```
Result: 20 total worker slots (5 pods × 4 slots)

### Vertical Scaling (Bigger Workers)
```yaml
# Increase memory per worker
supervisor:
  replicaCount: 3
  memoryConfig:
    memoryPerWorker: "2Gi"  # Was 1Gi, now 2Gi
```
Result: Larger workers for memory-intensive topologies

### Mixed Scaling
```yaml
# Different supervisor "sizes"
supervisor-small:
  replicaCount: 2
  memoryConfig:
    slotsPerSupervisor: 8
    memoryPerWorker: "512Mi"
    
supervisor-large:
  replicaCount: 1
  memoryConfig:
    slotsPerSupervisor: 4
    memoryPerWorker: "4Gi"
```

## Topology Integration

Topologies can specify their resource requirements:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
spec:
  topology: "com.example.MyTopology"
  workerCount: 6
  config:
    topology.worker.max.heap.size.mb: 1536  # Override if needed
    topology.component.resources.onheap.memory.mb: 256
```

The controller validates:
- Enough worker slots exist (6 workers needed)
- Worker heap can accommodate topology requirements

## Migration Path

1. **Current Setup**: Manual resource configuration
2. **Step 1**: Add memoryConfig section with mode: "manual"
3. **Step 2**: Test auto mode with same values
4. **Step 3**: Switch to auto mode in production

## Best Practices

1. **Start Conservative**: Begin with more memory than needed, optimize down
2. **Monitor Actual Usage**: Use metrics to right-size workers
3. **Leave Headroom**: 25-30% overhead for JVM is recommended
4. **Test Thoroughly**: Validate with actual topology workloads

## Common Configurations

| Use Case | Workers | Memory/Worker | Overhead | Container |
|----------|---------|---------------|----------|-----------|
| Dev/Test | 2       | 512Mi        | 25%      | 1.25Gi    |
| Small    | 4       | 1Gi          | 25%      | 5Gi       |
| Standard | 6       | 1.5Gi        | 30%      | 11.7Gi    |
| Large    | 8       | 2Gi          | 30%      | 20.8Gi    |
| XLarge   | 4       | 4Gi          | 35%      | 21.6Gi    |

## Troubleshooting

**Problem**: OutOfMemoryError in workers
- Increase `memoryPerWorker` or reduce `slotsPerSupervisor`

**Problem**: Container OOMKilled
- Increase `memoryOverheadPercent` (try 35-40%)

**Problem**: Topology won't deploy (insufficient resources)
- Check total cluster capacity vs topology requirements
- Add more supervisor replicas

**Problem**: Poor performance
- Workers may be too small - increase `memoryPerWorker`
- Check if CPU is the bottleneck - increase `cpuPerWorker`