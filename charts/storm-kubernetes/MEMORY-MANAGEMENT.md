# Storm Kubernetes Memory Management Options

## Memory Hierarchy Overview

Storm memory management involves multiple layers:

```
Container Resources (Kubernetes)
    └── JVM Process Memory
           ├── Heap Memory (-Xmx)
           └── Non-Heap Memory
                 ├── Metaspace
                 ├── Direct Memory
                 └── Thread Stacks
```

## Key Memory Configurations

### 1. Container Level (Kubernetes)
- `supervisor.resources.limits.memory`: Total container memory
- `supervisor.resources.requests.memory`: Guaranteed memory

### 2. Storm Supervisor Level
- `supervisor.memory.capacity.mb`: Memory capacity advertised to Nimbus
- `supervisor.cpu.capacity`: CPU capacity (400 = 4 cores)

### 3. Worker Level
- `worker.heap.memory.mb`: Default heap for each worker (default: 768MB)
- `topology.worker.max.heap.size.mb`: Max heap size per worker
- `worker.childopts`: JVM options including -Xmx

### 4. Topology Level
- `topology.component.resources.onheap.memory.mb`: Per-component heap
- `topology.component.resources.offheap.memory.mb`: Per-component off-heap
- `topology.worker.max.heap.size.mb`: Override worker heap for topology

## Option 1: Fixed Memory Profiles (Simple)

Create predefined memory profiles that automatically configure all layers:

```yaml
supervisor:
  memoryProfile: "medium"  # nano, micro, small, medium, large, xlarge
  slotsPerSupervisor: 4
```

### Profile Definitions:

| Profile | Container Memory | JVM Heap | Worker Heap | Slots | Total Worker Memory |
|---------|-----------------|----------|-------------|-------|-------------------|
| nano    | 2Gi            | 1.5Gi    | 256MB       | 4     | 1Gi               |
| micro   | 4Gi            | 3Gi      | 512MB       | 4     | 2Gi               |
| small   | 8Gi            | 6Gi      | 768MB       | 6     | 4.5Gi             |
| medium  | 16Gi           | 12Gi     | 1Gi         | 8     | 8Gi               |
| large   | 32Gi           | 24Gi     | 2Gi         | 10    | 20Gi              |
| xlarge  | 64Gi           | 48Gi     | 4Gi         | 10    | 40Gi              |

### Implementation:
```yaml
{{- define "storm.supervisor.memoryProfile" -}}
{{- $profiles := dict 
  "nano" (dict 
    "containerMemory" "2Gi"
    "workerHeap" 256
    "supervisorCapacity" 2048
  )
  "micro" (dict 
    "containerMemory" "4Gi"
    "workerHeap" 512
    "supervisorCapacity" 4096
  )
  # ... etc
}}
{{- end }}
```

**Pros:**
- Simple to use
- Prevents misconfiguration
- Good defaults for common use cases

**Cons:**
- Less flexible
- May waste resources
- Doesn't account for topology-specific needs

## Option 2: Automatic Calculation (Smart Defaults)

Calculate memory settings based on slots and a per-worker memory target:

```yaml
supervisor:
  slotsPerSupervisor: 4
  memoryPerWorker: 1Gi  # Target memory per worker
  memoryOverheadPercent: 25  # JVM overhead
```

### Auto-calculation:
- Worker heap = memoryPerWorker * 0.8 (leave 20% for off-heap)
- Total worker memory = slotsPerSupervisor * memoryPerWorker
- Container memory = Total worker memory * (1 + memoryOverheadPercent/100)
- Supervisor capacity = Total worker memory

### Implementation:
```yaml
supervisor:
  resources:
    requests:
      memory: {{ mul .Values.supervisor.slotsPerSupervisor .Values.supervisor.memoryPerWorker | mul 1.25 }}
    limits:
      memory: {{ mul .Values.supervisor.slotsPerSupervisor .Values.supervisor.memoryPerWorker | mul 1.25 }}
  extraConfig:
    supervisor.memory.capacity.mb: {{ mul .Values.supervisor.slotsPerSupervisor .Values.supervisor.memoryPerWorker | div 1048576 }}
    worker.heap.memory.mb: {{ .Values.supervisor.memoryPerWorker | mul 0.8 | div 1048576 }}
```

**Pros:**
- Flexible sizing
- Efficient resource usage
- Easy to scale

**Cons:**
- More complex configuration
- Requires understanding of memory model

## Option 3: Full Manual Control (Expert Mode)

Expose all memory settings directly:

```yaml
supervisor:
  resources:
    requests:
      memory: "16Gi"
      cpu: "4"
    limits:
      memory: "16Gi"
      cpu: "4"
  slotsPerSupervisor: 8
  extraConfig:
    supervisor.memory.capacity.mb: 12288
    supervisor.cpu.capacity: 400
    worker.heap.memory.mb: 1024
    topology.worker.max.heap.size.mb: 1536
```

**Pros:**
- Complete control
- Can optimize for specific workloads
- Supports advanced tuning

**Cons:**
- Easy to misconfigure
- Requires deep Storm knowledge
- No safety guardrails

## Recommendation: Hybrid Approach (Option 2 Enhanced)

Combine automatic calculation with override capabilities:

```yaml
supervisor:
  # Memory configuration
  memoryConfig:
    mode: "auto"  # auto, manual
    slotsPerSupervisor: 4
    memoryPerWorker: "1Gi"
    memoryOverheadPercent: 25
    
  # Optional overrides (used when mode=manual)
  resources:
    requests:
      memory: ""  # Override auto-calculation
    limits:
      memory: ""  # Override auto-calculation
      
  # Storm configuration
  extraConfig:
    # These are auto-calculated unless manually set
    # supervisor.memory.capacity.mb: 4096
    # worker.heap.memory.mb: 820
```

### Helper Template:
```yaml
{{- define "storm.supervisor.memory" -}}
{{- if eq .Values.supervisor.memoryConfig.mode "auto" -}}
  {{- $workerMemoryMB := .Values.supervisor.memoryConfig.memoryPerWorker | trimSuffix "Gi" | mul 1024 -}}
  {{- $totalWorkerMemory := mul $workerMemoryMB .Values.supervisor.memoryConfig.slotsPerSupervisor -}}
  {{- $overheadMultiplier := add 1 (div .Values.supervisor.memoryConfig.memoryOverheadPercent 100.0) -}}
  {{- $containerMemory := mul $totalWorkerMemory $overheadMultiplier | int -}}
  {{- dict "containerMemory" (printf "%dMi" $containerMemory) "supervisorCapacity" $totalWorkerMemory "workerHeap" (mul $workerMemoryMB 0.8 | int) -}}
{{- else -}}
  {{- dict "containerMemory" .Values.supervisor.resources.limits.memory "supervisorCapacity" .Values.supervisor.extraConfig.supervisor.memory.capacity.mb "workerHeap" .Values.supervisor.extraConfig.worker.heap.memory.mb -}}
{{- end -}}
{{- end -}}
```

### Benefits:
1. **Safe defaults**: Auto mode prevents common mistakes
2. **Flexibility**: Can override for special cases
3. **Scaling**: Easy horizontal scaling by changing replicas
4. **Topology-aware**: Topologies can still override worker memory
5. **Monitoring-friendly**: Clear relationship between settings

### Usage Examples:

#### Example 1: Development Environment
```yaml
supervisor:
  replicaCount: 1
  memoryConfig:
    mode: "auto"
    slotsPerSupervisor: 2
    memoryPerWorker: "512Mi"
    memoryOverheadPercent: 25
```
Results in:
- Container: 1.25Gi memory
- 2 worker slots @ 512Mi each
- Worker heap: 410MB

#### Example 2: Production Environment
```yaml
supervisor:
  replicaCount: 3
  memoryConfig:
    mode: "auto"
    slotsPerSupervisor: 8
    memoryPerWorker: "2Gi"
    memoryOverheadPercent: 30
```
Results in:
- Container: 20.8Gi memory per pod
- 8 worker slots @ 2Gi each
- Worker heap: 1.6GB
- Total cluster: 24 worker slots across 3 pods

#### Example 3: High-Memory Topology
```yaml
supervisor:
  replicaCount: 2
  memoryConfig:
    mode: "manual"
  resources:
    limits:
      memory: "64Gi"
  slotsPerSupervisor: 4
  extraConfig:
    supervisor.memory.capacity.mb: 49152  # 48Gi
    worker.heap.memory.mb: 10240  # 10Gi per worker
```

### Integration with Topology Deployment

The StormTopology controller can validate memory requirements:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
spec:
  workerCount: 4
  resources:
    memoryPerWorker: "2Gi"  # Validates against supervisor capacity
    cpuPerWorker: "1"
```

The controller checks:
1. Total memory needed <= Total supervisor capacity
2. Memory per worker <= Max worker heap size
3. Available slots >= Requested workers