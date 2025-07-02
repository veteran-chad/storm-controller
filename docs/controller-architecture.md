# Storm Kubernetes Controller Architecture

## Overview

The Storm Kubernetes Controller is a Kubernetes operator that manages Apache Storm topologies on Kubernetes. It follows the operator pattern to provide declarative management of Storm clusters and topologies through Custom Resource Definitions (CRDs).

The controller is designed to be **namespace-scoped**, meaning each controller instance manages a single Storm cluster within a specific namespace. This design enables multi-tenancy and simplified RBAC management.

## Architecture Components

### Custom Resource Definitions (CRDs)

The controller manages three primary resources:

1. **StormCluster** - Represents a reference to an existing Storm cluster deployment
2. **StormTopology** - Defines Storm topologies to be deployed on the cluster
3. **StormWorkerPool** - Manages dedicated worker pools for topologies (partial implementation)

### Controller Components

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        subgraph "Control Plane"
            API[Kubernetes API Server]
            ETCD[ETCD Storage]
        end
        
        subgraph "Storm Controller"
            CM[Controller Manager]
            SCR[StormCluster Reconciler]
            STR[StormTopology Reconciler]
            SWR[StormWorkerPool Reconciler]
            SC[Storm Client]
            JE[JAR Extractor]
            MS[Metrics Server]
        end
        
        subgraph "Storm Cluster"
            N[Nimbus]
            UI[Storm UI]
            S1[Supervisor 1]
            S2[Supervisor 2]
            S3[Supervisor N]
        end
        
        subgraph "Resources"
            SCRes[StormCluster CR]
            STRes[StormTopology CR]
            SWRes[StormWorkerPool CR]
        end
    end
    
    API --> CM
    CM --> SCR
    CM --> STR
    CM --> SWR
    
    SCR --> SC
    STR --> SC
    STR --> JE
    
    SC --> N
    SC --> UI
    
    SCR --> MS
    STR --> MS
    SWR --> MS
    
    SCRes --> API
    STRes --> API
    SWRes --> API
    
    ETCD --> API
```

## Controller Workflow

### High-Level Controller Flow

```mermaid
flowchart TB
    Start([Controller Start])
    Init[Initialize Components]
    LE{Leader Election}
    Watch[Watch Resources]
    Event{Resource Event}
    Reconcile[Reconcile Resource]
    Update[Update Status]
    Metrics[Update Metrics]
    
    Start --> Init
    Init --> LE
    LE -->|Not Leader| LE
    LE -->|Leader| Watch
    Watch --> Event
    Event -->|Create/Update/Delete| Reconcile
    Reconcile --> Update
    Update --> Metrics
    Metrics --> Watch
```

### StormCluster Reconciliation Flow

```mermaid
flowchart TD
    Start([Reconcile Request])
    Fetch[Fetch StormCluster Resource]
    Check{Resource Exists?}
    Delete{Deletion?}
    Query[Query Storm REST API]
    UpdateStatus[Update Cluster Status]
    UpdateMetrics[Update Prometheus Metrics]
    SetConditions[Set Availability Conditions]
    Requeue[Requeue after 60s]
    End([End])
    
    Start --> Fetch
    Fetch --> Check
    Check -->|No| End
    Check -->|Yes| Delete
    Delete -->|Yes| End
    Delete -->|No| Query
    Query --> UpdateStatus
    UpdateStatus --> UpdateMetrics
    UpdateMetrics --> SetConditions
    SetConditions --> Requeue
    Requeue --> End
```

### StormTopology Reconciliation Flow

```mermaid
flowchart TD
    Start([Reconcile Request])
    Fetch[Fetch StormTopology Resource]
    CheckDeletion{Deletion Marked?}
    KillTopology[Kill Topology via CLI]
    RemoveFinalizer[Remove Finalizer]
    CheckCluster[Verify StormCluster Health]
    ClusterHealthy{Cluster Healthy?}
    VersionChange{Version Changed?}
    KillOldVersion[Kill Old Topology]
    WaitRemoval[Wait for Removal]
    DownloadJAR[Download/Extract JAR]
    SubmitTopology[Submit via Storm CLI]
    UpdateStatus[Update Topology Status]
    UpdateMetrics[Update Metrics]
    Requeue[Requeue for Monitoring]
    Error[Set Error Status]
    End([End])
    
    Start --> Fetch
    Fetch --> CheckDeletion
    CheckDeletion -->|Yes| KillTopology
    KillTopology --> RemoveFinalizer
    RemoveFinalizer --> End
    CheckDeletion -->|No| CheckCluster
    CheckCluster --> ClusterHealthy
    ClusterHealthy -->|No| Error
    ClusterHealthy -->|Yes| VersionChange
    VersionChange -->|Yes| KillOldVersion
    KillOldVersion --> WaitRemoval
    WaitRemoval --> DownloadJAR
    VersionChange -->|No| DownloadJAR
    DownloadJAR --> SubmitTopology
    SubmitTopology --> UpdateStatus
    UpdateStatus --> UpdateMetrics
    UpdateMetrics --> Requeue
    Requeue --> End
    Error --> End
```

## Resource State Diagrams

### StormCluster State Diagram

```mermaid
stateDiagram-v2
    [*] --> Created: Resource Created
    Created --> Initializing: First Reconciliation
    Initializing --> Connecting: Contacting Storm API
    Connecting --> Available: Connection Successful
    Connecting --> Unavailable: Connection Failed
    Available --> Monitoring: Periodic Health Check
    Monitoring --> Available: Healthy
    Monitoring --> Degraded: Some Issues
    Monitoring --> Unavailable: Connection Lost
    Degraded --> Monitoring: Continue Monitoring
    Unavailable --> Connecting: Retry Connection
    Available --> Deleting: Deletion Requested
    Degraded --> Deleting: Deletion Requested
    Unavailable --> Deleting: Deletion Requested
    Deleting --> [*]: Finalizer Removed
```

### StormTopology State Diagram

```mermaid
stateDiagram-v2
    [*] --> Created: Resource Created
    Created --> Pending: Awaiting Cluster
    Pending --> Downloading: Cluster Available
    Downloading --> Downloaded: JAR Retrieved
    Downloaded --> Submitting: Submit to Storm
    Submitting --> Running: Submission Success
    Submitting --> Failed: Submission Error
    Running --> Updating: Version Change
    Updating --> Killing: Kill Old Version
    Killing --> Waiting: Await Removal
    Waiting --> Downloading: Old Version Gone
    Running --> Suspending: Suspend Requested
    Suspending --> Suspended: Suspension Complete
    Suspended --> Resuming: Resume Requested
    Resuming --> Running: Resume Complete
    Running --> Terminating: Deletion Requested
    Failed --> Terminating: Deletion Requested
    Suspended --> Terminating: Deletion Requested
    Terminating --> [*]: Finalizer Removed
```

### StormWorkerPool State Diagram

```mermaid
stateDiagram-v2
    [*] --> Created: Resource Created
    Created --> Pending: Awaiting Implementation
    Pending --> Active: Pool Created
    Active --> Scaling: HPA Triggered
    Scaling --> Active: Scaling Complete
    Active --> Updating: Configuration Change
    Updating --> Active: Update Complete
    Active --> Deleting: Deletion Requested
    Deleting --> [*]: Resources Cleaned
    
    note right of Pending: Full implementation pending
```

## JAR Extraction Flow

```mermaid
flowchart TD
    Start([JAR Source Specified])
    Type{Source Type?}
    URL[Download from URL]
    Container[Extract from Container]
    ConfigMap[Load from ConfigMap]
    Secret[Load from Secret]
    S3[Download from S3]
    Cache[Cache JAR Locally]
    Checksum{Checksum Provided?}
    Validate[Validate Checksum]
    Valid{Valid?}
    Return[Return JAR Path]
    Error[Return Error]
    
    Start --> Type
    Type -->|URL| URL
    Type -->|Container| Container
    Type -->|ConfigMap| ConfigMap
    Type -->|Secret| Secret
    Type -->|S3| S3
    
    URL --> Cache
    Container --> Cache
    ConfigMap --> Cache
    Secret --> Cache
    S3 --> Cache
    
    Cache --> Checksum
    Checksum -->|Yes| Validate
    Checksum -->|No| Return
    Validate --> Valid
    Valid -->|Yes| Return
    Valid -->|No| Error
```

## Key Design Decisions

### 1. Namespace-Scoped Architecture

The controller is designed to manage a single Storm cluster per namespace:

- **Isolation**: Each namespace can have its own Storm cluster
- **Multi-tenancy**: Different teams can manage their own clusters
- **RBAC**: Simplified permission management
- **Resource Limits**: Per-namespace resource quotas

### 2. Reference-Based Cluster Management

StormCluster resources don't create Storm infrastructure:

- **Flexibility**: Use any deployment method (Helm, manifests, operators)
- **Separation of Concerns**: Infrastructure vs. application management
- **Compatibility**: Works with existing Storm deployments

### 3. CLI-Based Topology Submission

Currently uses Storm CLI instead of Thrift API:

- **Simplicity**: Easier implementation and debugging
- **Compatibility**: Works with all Storm versions
- **Trade-off**: Less efficient than direct Thrift
- **Future**: Plans to migrate to Thrift API

### 4. Periodic Reconciliation

Regular status updates ensure eventual consistency:

- **Health Checks**: Every 60 seconds for clusters
- **Topology Monitoring**: Continuous state verification
- **External Changes**: Detects manual interventions

## Metrics and Observability

The controller exposes comprehensive Prometheus metrics:

```mermaid
graph LR
    Controller[Storm Controller]
    Prometheus[Prometheus Server]
    Grafana[Grafana]
    AlertManager[AlertManager]
    
    Controller -->|/metrics| Prometheus
    Prometheus --> Grafana
    Prometheus --> AlertManager
    
    subgraph "Metrics Exposed"
        CM[Cluster Metrics]
        TM[Topology Metrics]
        WM[Worker Pool Metrics]
        OM[Operational Metrics]
    end
```

### Metric Categories

1. **Cluster Metrics**
   - Supervisor count
   - Slot availability (total/used/free)
   - Cluster health status

2. **Topology Metrics**
   - Submission attempts/successes/failures
   - Running topology count
   - Version update frequency

3. **Worker Pool Metrics**
   - Pool size and utilization
   - Scaling events

4. **Operational Metrics**
   - Reconciliation duration
   - Error rates
   - API call latencies

## Security Considerations

### RBAC Requirements

The controller requires specific Kubernetes permissions:

```yaml
- apiGroups: ["storm.apache.org"]
  resources: ["stormclusters", "stormtopologies", "stormworkerpools"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets", "pods"]
  verbs: ["get", "list", "watch"]
```

### Network Security

- Controller → Storm API: HTTP/HTTPS connections
- Topology submission: Via Storm CLI (requires network access to Nimbus)
- Metrics exposure: Secured endpoint for Prometheus scraping

## Limitations and Future Enhancements

### Current Limitations

1. **JAR Sources**: Limited to URLs and container images
2. **Authentication**: No built-in Storm authentication support
3. **Worker Pools**: Basic implementation only
4. **API Integration**: CLI-based instead of Thrift API
5. **Cluster Management**: Single cluster per controller

### Planned Enhancements

1. **Thrift API Integration**: Direct API calls for efficiency
2. **Advanced Worker Pools**: Full HPA and resource management
3. **Multi-Cluster Support**: Single controller managing multiple clusters
4. **Enhanced Security**: Storm authentication and authorization
5. **GitOps Integration**: Better support for declarative deployments

## Deployment Architecture

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        subgraph "storm-system namespace"
            HC[Helm Chart]
            SC[Storm Cluster]
            CO[Storm Controller]
        end
        
        subgraph "application namespace"
            ST1[StormTopology 1]
            ST2[StormTopology 2]
            SCR[StormCluster Reference]
        end
    end
    
    HC --> SC
    HC --> CO
    CO --> SCR
    SCR --> SC
    ST1 --> SCR
    ST2 --> SCR
```

## Conclusion

The Storm Kubernetes Controller provides a cloud-native way to manage Apache Storm topologies on Kubernetes. Its architecture follows Kubernetes best practices while maintaining flexibility and extensibility. The namespace-scoped design and reference-based cluster management make it suitable for both simple deployments and complex multi-tenant environments.