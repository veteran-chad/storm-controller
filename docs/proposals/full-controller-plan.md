# Storm Controller Enhancement and Thrift Migration Plan

## Executive Summary

This document outlines a comprehensive plan to enhance the Storm Kubernetes controller architecture and migrate from CLI/REST interactions to a native Thrift client implementation. The plan maintains the current separation of controllers while addressing existing gaps and improving coordination between resources.

## Current State

### Architecture Overview
- **Three separate controllers** managing distinct CRDs:
  - `StormCluster` - monitors existing clusters (read-only)
  - `StormTopology` - manages topology lifecycle
  - `StormWorkerPool` - stub implementation for worker management
- Controllers communicate through Kubernetes resource references
- Uses Storm CLI for critical operations (submit, kill)
- REST API for monitoring operations

### Key Limitations
1. StormCluster controller cannot provision clusters
2. StormWorkerPool controller is not implemented
3. No cross-resource watches for coordination
4. Dependency on Storm CLI in controller image
5. Limited error handling for Storm API interactions

## Proposed Architecture

### Design Principles
1. **Maintain separation of concerns** - each controller has distinct responsibilities
2. **Enable loose coupling** - controllers communicate via K8s resources
3. **Support independent scaling** - controllers can scale independently
4. **Implement progressive enhancement** - migrate incrementally

### Enhanced Controller Responsibilities

#### StormCluster Controller
- **Provision and manage** Storm cluster infrastructure
- Monitor cluster health and availability
- Handle cluster scaling and upgrades
- Manage Nimbus leader election awareness
- Report detailed cluster metrics

#### StormTopology Controller
- Submit and manage topology lifecycle
- Handle version-based updates
- Implement capacity checking before submission
- Watch referenced clusters for changes
- Manage topology placement hints

#### StormWorkerPool Controller
- Create and manage worker deployments
- Implement horizontal autoscaling
- Handle worker-specific configurations
- Monitor worker health and performance
- Coordinate with topology requirements

## Implementation Phases

### Phase 1: Thrift Client Infrastructure (Week 1-2)

#### 1.1 Setup Thrift Dependencies
```bash
# Add to go.mod
go get github.com/apache/thrift/lib/go/thrift

# Install Thrift compiler
brew install thrift  # or appropriate package manager
```

#### 1.2 Generate Go Client Code
```bash
# Create directory structure
mkdir -p src/pkg/storm/thrift/generated

# Generate Go code from storm.thrift
thrift --gen go:package_prefix=github.com/veteran-chad/storm-controller/pkg/storm/thrift/generated/ \
  -out src/pkg/storm/thrift/generated \
  external/storm/storm-client/src/storm.thrift
```

#### 1.3 Create Thrift Client Infrastructure

**File: `src/pkg/storm/thrift/client.go`**
```go
package thrift

import (
    "context"
    "sync"
    "time"
    
    "github.com/apache/thrift/lib/go/thrift"
    "github.com/veteran-chad/storm-controller/pkg/storm/thrift/generated/storm"
)

type ClientPool struct {
    mu          sync.RWMutex
    connections map[string]*pooledConnection
    config      PoolConfig
}

type PoolConfig struct {
    MaxConnections   int
    ConnectTimeout   time.Duration
    RequestTimeout   time.Duration
    IdleTimeout      time.Duration
    RetryPolicy      RetryPolicy
}

type pooledConnection struct {
    client     *storm.NimbusClient
    transport  thrift.TTransport
    lastUsed   time.Time
    inUse      bool
}

// GetClient returns a Thrift client for the specified cluster
func (p *ClientPool) GetClient(clusterName string) (*storm.NimbusClient, error) {
    // Implementation for connection pooling
    // Includes retry logic and circuit breaking
}
```

**File: `src/pkg/storm/thrift/interface.go`**
```go
package thrift

import (
    "context"
    "github.com/veteran-chad/storm-controller/api/v1beta1"
)

type StormClient interface {
    // Topology Operations
    SubmitTopology(ctx context.Context, name string, jarPath string, config string, topology *storm.StormTopology) error
    KillTopology(ctx context.Context, name string) error
    KillTopologyWithOpts(ctx context.Context, name string, waitTime int32) error
    ActivateTopology(ctx context.Context, name string) error
    DeactivateTopology(ctx context.Context, name string) error
    RebalanceTopology(ctx context.Context, name string, options *storm.RebalanceOptions) error
    
    // Topology Information
    GetTopologyInfo(ctx context.Context, name string) (*storm.TopologyInfo, error)
    GetTopologySummaryByName(ctx context.Context, name string) (*storm.TopologySummary, error)
    GetTopologyConf(ctx context.Context, id string) (map[string]string, error)
    
    // Cluster Information
    GetClusterInfo(ctx context.Context) (*storm.ClusterSummary, error)
    GetNimbusConf(ctx context.Context) (map[string]string, error)
    GetSupervisorPageInfo(ctx context.Context, id string, host string, includeSystem bool) (*storm.SupervisorPageInfo, error)
    
    // File Operations
    UploadJar(ctx context.Context, localPath string) (string, error)
    DownloadJar(ctx context.Context, path string) ([]byte, error)
}
```

### Phase 2: Controller Enhancements (Week 3-4)

#### 2.1 Enhance StormCluster Controller

**Add Cluster Provisioning:**
```go
// Add to StormClusterReconciler
func (r *StormClusterReconciler) reconcileClusterResources(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
    // Create Zookeeper StatefulSet
    if err := r.reconcileZookeeper(ctx, cluster); err != nil {
        return err
    }
    
    // Create Nimbus Deployment
    if err := r.reconcileNimbus(ctx, cluster); err != nil {
        return err
    }
    
    // Create Supervisor DaemonSet/Deployment
    if err := r.reconcileSupervisors(ctx, cluster); err != nil {
        return err
    }
    
    // Create UI Deployment (optional)
    if cluster.Spec.UI.Enabled {
        if err := r.reconcileUI(ctx, cluster); err != nil {
            return err
        }
    }
    
    return nil
}
```

**Add Cross-Resource Tracking:**
```go
func (r *StormClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&stormv1beta1.StormCluster{}).
        Owns(&appsv1.StatefulSet{}).
        Owns(&appsv1.Deployment{}).
        Owns(&appsv1.DaemonSet{}).
        Owns(&corev1.Service{}).
        Watches(&source.Kind{Type: &stormv1beta1.StormTopology{}}, 
            handler.EnqueueRequestsFromMapFunc(r.findClusterForTopology)).
        Complete(r)
}
```

#### 2.2 Enhance StormTopology Controller

**Integrate Thrift Client:**
```go
func (r *StormTopologyReconciler) submitTopologyViaThrift(ctx context.Context, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) error {
    // Get Thrift client for cluster
    client, err := r.ThriftPool.GetClient(cluster.Name)
    if err != nil {
        return fmt.Errorf("failed to get thrift client: %w", err)
    }
    
    // Upload JAR file
    uploadedPath, err := r.uploadJar(ctx, client, topology)
    if err != nil {
        return fmt.Errorf("failed to upload jar: %w", err)
    }
    
    // Prepare topology configuration
    config, err := r.prepareTopologyConfig(topology)
    if err != nil {
        return fmt.Errorf("failed to prepare config: %w", err)
    }
    
    // Build StormTopology structure
    stormTopology, err := r.buildStormTopology(topology)
    if err != nil {
        return fmt.Errorf("failed to build topology: %w", err)
    }
    
    // Submit topology
    err = client.SubmitTopology(ctx, topology.Spec.Topology.Name, uploadedPath, config, stormTopology)
    if err != nil {
        return fmt.Errorf("failed to submit topology: %w", err)
    }
    
    return nil
}
```

**Add Cluster Watch:**
```go
func (r *StormTopologyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&stormv1beta1.StormTopology{}).
        Watches(&source.Kind{Type: &stormv1beta1.StormCluster{}}, 
            handler.EnqueueRequestsFromMapFunc(r.findTopologiesForCluster)).
        WithEventFilter(predicate.Or(
            predicate.GenerationChangedPredicate{},
            predicate.AnnotationChangedPredicate{},
        )).
        Complete(r)
}
```

#### 2.3 Implement StormWorkerPool Controller

```go
package controllers

import (
    "context"
    "fmt"
    
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    
    stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

type StormWorkerPoolReconciler struct {
    client.Client
    Scheme      *runtime.Scheme
    ThriftPool  *thrift.ClientPool
}

func (r *StormWorkerPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    // Get WorkerPool resource
    workerPool := &stormv1beta1.StormWorkerPool{}
    if err := r.Get(ctx, req.NamespacedName, workerPool); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // Get referenced topology
    topology := &stormv1beta1.StormTopology{}
    if err := r.Get(ctx, client.ObjectKey{
        Name:      workerPool.Spec.TopologyRef,
        Namespace: workerPool.Namespace,
    }, topology); err != nil {
        return ctrl.Result{}, err
    }
    
    // Get referenced cluster
    cluster := &stormv1beta1.StormCluster{}
    clusterRef := workerPool.Spec.ClusterRef
    if clusterRef == "" {
        clusterRef = topology.Spec.ClusterRef
    }
    if err := r.Get(ctx, client.ObjectKey{
        Name:      clusterRef,
        Namespace: workerPool.Namespace,
    }, cluster); err != nil {
        return ctrl.Result{}, err
    }
    
    // Reconcile worker deployment
    deployment := r.buildWorkerDeployment(workerPool, topology, cluster)
    if err := r.reconcileDeployment(ctx, deployment, workerPool); err != nil {
        return ctrl.Result{}, err
    }
    
    // Setup HPA if autoscaling is enabled
    if workerPool.Spec.Autoscaling != nil && workerPool.Spec.Autoscaling.Enabled {
        if err := r.reconcileHPA(ctx, workerPool); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // Update status
    return ctrl.Result{}, r.updateStatus(ctx, workerPool, deployment)
}
```

### Phase 2.5: Implement Testable State Machines (Week 4)

#### Design Principle: Testable Business Logic

All controller business logic should be implemented using testable state machines that are decoupled from Kubernetes client operations. This enables thorough unit testing without requiring a Kubernetes cluster.

#### 2.5.1 State Machine Architecture

**File: `src/pkg/statemachine/interface.go`**
```go
package statemachine

import (
    "context"
    stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

// StateMachine defines the interface for resource state management
type StateMachine interface {
    // ValidateTransition checks if a state transition is valid
    ValidateTransition(from, to string) error
    
    // GetNextState determines the next state based on current state and conditions
    GetNextState(current string, conditions StateConditions) (string, error)
    
    // GetRequiredActions returns actions needed for a state transition
    GetRequiredActions(from, to string) []StateAction
}

// StateConditions represents external conditions affecting state
type StateConditions struct {
    ResourceExists   bool
    ResourceHealthy  bool
    DependenciesMet  bool
    CustomConditions map[string]interface{}
}

// StateAction represents an action to be performed
type StateAction struct {
    Type        string
    Description string
    Parameters  map[string]interface{}
}
```

**File: `src/pkg/statemachine/topology_state_machine.go`**
```go
package statemachine

import (
    "fmt"
)

// TopologyStateMachine implements state transitions for StormTopology
type TopologyStateMachine struct {
    validTransitions map[string][]string
}

func NewTopologyStateMachine() *TopologyStateMachine {
    return &TopologyStateMachine{
        validTransitions: map[string][]string{
            "":            {"Pending"},
            "Pending":     {"Provisioning", "Failed"},
            "Provisioning": {"Running", "Failed"},
            "Running":     {"Updating", "Suspended", "Terminating"},
            "Updating":    {"Running", "Failed"},
            "Suspended":   {"Running", "Terminating"},
            "Failed":      {"Pending", "Terminating"},
            "Terminating": {"Terminated"},
            "Terminated":  {},
        },
    }
}

func (sm *TopologyStateMachine) ValidateTransition(from, to string) error {
    validStates, ok := sm.validTransitions[from]
    if !ok {
        return fmt.Errorf("unknown state: %s", from)
    }
    
    for _, valid := range validStates {
        if valid == to {
            return nil
        }
    }
    
    return fmt.Errorf("invalid transition from %s to %s", from, to)
}

func (sm *TopologyStateMachine) GetNextState(current string, conditions StateConditions) (string, error) {
    switch current {
    case "":
        return "Pending", nil
        
    case "Pending":
        if !conditions.DependenciesMet {
            return "Failed", nil
        }
        return "Provisioning", nil
        
    case "Provisioning":
        if !conditions.ResourceExists {
            return "Failed", nil
        }
        if conditions.ResourceHealthy {
            return "Running", nil
        }
        return current, nil // Stay in Provisioning
        
    case "Running":
        if !conditions.ResourceHealthy {
            return "Failed", nil
        }
        // Check for version changes or other update conditions
        if updateNeeded, ok := conditions.CustomConditions["versionChanged"].(bool); ok && updateNeeded {
            return "Updating", nil
        }
        return current, nil
        
    default:
        return current, nil
    }
}

func (sm *TopologyStateMachine) GetRequiredActions(from, to string) []StateAction {
    actions := []StateAction{}
    
    switch {
    case from == "Pending" && to == "Provisioning":
        actions = append(actions, StateAction{
            Type:        "DownloadJAR",
            Description: "Download topology JAR file",
        })
        actions = append(actions, StateAction{
            Type:        "ValidateTopology",
            Description: "Validate topology configuration",
        })
        
    case from == "Provisioning" && to == "Running":
        actions = append(actions, StateAction{
            Type:        "SubmitTopology",
            Description: "Submit topology to Storm cluster",
        })
        
    case from == "Running" && to == "Updating":
        actions = append(actions, StateAction{
            Type:        "KillTopology",
            Description: "Kill existing topology version",
        })
        actions = append(actions, StateAction{
            Type:        "WaitForRemoval",
            Description: "Wait for topology to be removed",
        })
        
    case to == "Terminating":
        actions = append(actions, StateAction{
            Type:        "KillTopology",
            Description: "Kill topology in Storm cluster",
        })
        actions = append(actions, StateAction{
            Type:        "CleanupResources",
            Description: "Clean up cached resources",
        })
    }
    
    return actions
}
```

#### 2.5.2 Controller Integration

**File: `src/controllers/stormtopology_controller_logic.go`**
```go
package controllers

import (
    "context"
    "github.com/veteran-chad/storm-controller/pkg/statemachine"
)

// TopologyReconcilerLogic handles business logic separate from K8s operations
type TopologyReconcilerLogic struct {
    stateMachine statemachine.StateMachine
}

func NewTopologyReconcilerLogic() *TopologyReconcilerLogic {
    return &TopologyReconcilerLogic{
        stateMachine: statemachine.NewTopologyStateMachine(),
    }
}

// DetermineActions determines what actions to take based on current and desired state
func (l *TopologyReconcilerLogic) DetermineActions(current, desired TopologyState) ([]ReconcileAction, error) {
    // Validate transition
    if err := l.stateMachine.ValidateTransition(current.Phase, desired.Phase); err != nil {
        return nil, err
    }
    
    // Get required actions
    stateActions := l.stateMachine.GetRequiredActions(current.Phase, desired.Phase)
    
    // Convert to reconcile actions
    actions := []ReconcileAction{}
    for _, sa := range stateActions {
        actions = append(actions, ReconcileAction{
            Type:       sa.Type,
            Parameters: sa.Parameters,
        })
    }
    
    return actions, nil
}

// CalculateDesiredState calculates the desired state based on spec and current conditions
func (l *TopologyReconcilerLogic) CalculateDesiredState(spec TopologySpec, status TopologyStatus, externalState ExternalState) (TopologyState, error) {
    conditions := statemachine.StateConditions{
        ResourceExists:  externalState.TopologyExists,
        ResourceHealthy: externalState.TopologyHealthy,
        DependenciesMet: externalState.ClusterReady,
        CustomConditions: map[string]interface{}{
            "versionChanged": spec.Version != status.DeployedVersion,
            "suspended":      spec.Suspend,
        },
    }
    
    nextState, err := l.stateMachine.GetNextState(status.Phase, conditions)
    if err != nil {
        return TopologyState{}, err
    }
    
    return TopologyState{
        Phase:   nextState,
        Version: spec.Version,
    }, nil
}
```

#### 2.5.3 Unit Testing State Machines

**File: `src/pkg/statemachine/topology_state_machine_test.go`**
```go
package statemachine_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/veteran-chad/storm-controller/pkg/statemachine"
)

func TestTopologyStateMachine_ValidateTransition(t *testing.T) {
    sm := statemachine.NewTopologyStateMachine()
    
    tests := []struct {
        name    string
        from    string
        to      string
        wantErr bool
    }{
        {
            name:    "valid transition from pending to provisioning",
            from:    "Pending",
            to:      "Provisioning",
            wantErr: false,
        },
        {
            name:    "invalid transition from running to pending",
            from:    "Running",
            to:      "Pending",
            wantErr: true,
        },
        {
            name:    "valid transition from failed to pending",
            from:    "Failed",
            to:      "Pending",
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := sm.ValidateTransition(tt.from, tt.to)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestTopologyStateMachine_GetNextState(t *testing.T) {
    sm := statemachine.NewTopologyStateMachine()
    
    tests := []struct {
        name       string
        current    string
        conditions statemachine.StateConditions
        want       string
    }{
        {
            name:    "pending with met dependencies",
            current: "Pending",
            conditions: statemachine.StateConditions{
                DependenciesMet: true,
            },
            want: "Provisioning",
        },
        {
            name:    "pending with unmet dependencies",
            current: "Pending",
            conditions: statemachine.StateConditions{
                DependenciesMet: false,
            },
            want: "Failed",
        },
        {
            name:    "running with version change",
            current: "Running",
            conditions: statemachine.StateConditions{
                ResourceHealthy: true,
                CustomConditions: map[string]interface{}{
                    "versionChanged": true,
                },
            },
            want: "Updating",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := sm.GetNextState(tt.current, tt.conditions)
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}

func TestTopologyStateMachine_GetRequiredActions(t *testing.T) {
    sm := statemachine.NewTopologyStateMachine()
    
    actions := sm.GetRequiredActions("Pending", "Provisioning")
    assert.Len(t, actions, 2)
    assert.Equal(t, "DownloadJAR", actions[0].Type)
    assert.Equal(t, "ValidateTopology", actions[1].Type)
    
    actions = sm.GetRequiredActions("Running", "Updating")
    assert.Len(t, actions, 2)
    assert.Equal(t, "KillTopology", actions[0].Type)
    assert.Equal(t, "WaitForRemoval", actions[1].Type)
}
```

#### 2.5.4 Benefits of This Approach

1. **Testability**: Business logic can be tested without K8s dependencies
2. **Clarity**: State transitions and actions are explicitly defined
3. **Maintainability**: Easy to add new states or modify transitions
4. **Debugging**: Clear state machine makes issues easier to diagnose
5. **Documentation**: State machine serves as living documentation

#### 2.5.5 Implementation for All Controllers

Similar state machines should be implemented for:

1. **ClusterStateMachine**: 
   - States: Pending → Provisioning → Running → Scaling → Upgrading → Terminating
   - Actions: CreateZookeeper, CreateNimbus, CreateSupervisors, etc.

2. **WorkerPoolStateMachine**:
   - States: Pending → Provisioning → Ready → Scaling → Updating → Terminating
   - Actions: CreateDeployment, UpdateReplicas, ConfigureHPA, etc.

Each controller's reconciliation loop becomes:
1. Gather current state from K8s resources
2. Query external state (Storm cluster status)
3. Calculate desired state using state machine
4. Determine required actions
5. Execute actions
6. Update resource status

This separation ensures all business logic is unit testable while keeping K8s operations in the controller layer.

### Phase 3: Add Coordination Layer (Week 5)

#### 3.1 Implement Admission Webhooks

**File: `src/webhooks/stormtopology_webhook.go`**
```go
package webhooks

import (
    "context"
    "fmt"
    
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
    
    stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

type StormTopologyWebhook struct {
    Client client.Client
}

func (w *StormTopologyWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
    topology := obj.(*stormv1beta1.StormTopology)
    
    // Validate cluster reference exists
    cluster := &stormv1beta1.StormCluster{}
    if err := w.Client.Get(ctx, client.ObjectKey{
        Name:      topology.Spec.ClusterRef,
        Namespace: topology.Namespace,
    }, cluster); err != nil {
        return fmt.Errorf("invalid cluster reference: %w", err)
    }
    
    // Validate cluster is ready
    if cluster.Status.Phase != "Running" {
        return fmt.Errorf("cluster %s is not ready (phase: %s)", cluster.Name, cluster.Status.Phase)
    }
    
    // Check cluster capacity
    if cluster.Status.FreeSlots < topology.Spec.Topology.Workers {
        return fmt.Errorf("insufficient cluster capacity: need %d workers, have %d free slots",
            topology.Spec.Topology.Workers, cluster.Status.FreeSlots)
    }
    
    return nil
}
```

#### 3.2 Cross-Resource Event Handling

**File: `src/pkg/coordinator/event_handler.go`**
```go
package coordinator

import (
    "context"
    "sigs.k8s.io/controller-runtime/pkg/handler"
    "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterEventHandler handles cluster events that affect topologies
type ClusterEventHandler struct {
    Client client.Client
}

func (h *ClusterEventHandler) HandleClusterUpdate(obj client.Object) []reconcile.Request {
    cluster := obj.(*stormv1beta1.StormCluster)
    
    // Find all topologies referencing this cluster
    topologyList := &stormv1beta1.StormTopologyList{}
    if err := h.Client.List(context.Background(), topologyList, 
        client.InNamespace(cluster.Namespace)); err != nil {
        return nil
    }
    
    var requests []reconcile.Request
    for _, topology := range topologyList.Items {
        if topology.Spec.ClusterRef == cluster.Name {
            requests = append(requests, reconcile.Request{
                NamespacedName: types.NamespacedName{
                    Name:      topology.Name,
                    Namespace: topology.Namespace,
                },
            })
        }
    }
    
    return requests
}
```

### Phase 4: JAR Management Enhancement (Week 6)

#### 4.1 Implement Thrift-based JAR Upload

```go
package thrift

import (
    "context"
    "io"
    "os"
    
    "github.com/veteran-chad/storm-controller/pkg/storm/thrift/generated/storm"
)

const (
    ChunkSize = 1024 * 1024 // 1MB chunks
)

func (c *ThriftStormClient) UploadJar(ctx context.Context, localPath string) (string, error) {
    // Open local JAR file
    file, err := os.Open(localPath)
    if err != nil {
        return "", fmt.Errorf("failed to open jar file: %w", err)
    }
    defer file.Close()
    
    // Get file info
    fileInfo, err := file.Stat()
    if err != nil {
        return "", fmt.Errorf("failed to stat jar file: %w", err)
    }
    
    // Begin file upload
    session, err := c.client.BeginFileUpload(ctx)
    if err != nil {
        return "", fmt.Errorf("failed to begin upload: %w", err)
    }
    
    // Upload in chunks
    buffer := make([]byte, ChunkSize)
    totalUploaded := int64(0)
    
    for {
        n, err := file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            return "", fmt.Errorf("failed to read jar file: %w", err)
        }
        
        chunk := buffer[:n]
        if err := c.client.UploadChunk(ctx, session, chunk); err != nil {
            return "", fmt.Errorf("failed to upload chunk: %w", err)
        }
        
        totalUploaded += int64(n)
        
        // Report progress
        if c.progressCallback != nil {
            c.progressCallback(totalUploaded, fileInfo.Size())
        }
    }
    
    // Finish upload
    if err := c.client.FinishFileUpload(ctx, session); err != nil {
        return "", fmt.Errorf("failed to finish upload: %w", err)
    }
    
    return session, nil
}
```

#### 4.2 Add Caching Layer

```go
package jar

import (
    "crypto/sha256"
    "fmt"
    "sync"
    "time"
)

type JarCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    maxSize int64
    ttl     time.Duration
}

type CacheEntry struct {
    Path         string
    UploadedPath string
    Size         int64
    SHA256       string
    LastUsed     time.Time
    RefCount     int
}

func (c *JarCache) GetOrUpload(ctx context.Context, localPath string, uploader JarUploader) (string, error) {
    // Calculate JAR hash
    hash, err := c.calculateHash(localPath)
    if err != nil {
        return "", err
    }
    
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Check if already cached
    if entry, ok := c.entries[hash]; ok {
        entry.LastUsed = time.Now()
        entry.RefCount++
        return entry.UploadedPath, nil
    }
    
    // Upload JAR
    uploadedPath, err := uploader.Upload(ctx, localPath)
    if err != nil {
        return "", err
    }
    
    // Add to cache
    c.entries[hash] = &CacheEntry{
        Path:         localPath,
        UploadedPath: uploadedPath,
        SHA256:       hash,
        LastUsed:     time.Now(),
        RefCount:     1,
    }
    
    // Clean old entries if needed
    go c.cleanOldEntries()
    
    return uploadedPath, nil
}
```

### Phase 5: Testing and Migration (Week 7-8)

#### 5.1 Testing Strategy

1. **Unit Tests**
   - **State Machine Tests** (no K8s dependencies required)
     - Test all valid state transitions
     - Test invalid transition rejection
     - Test action generation for each transition
     - Test state calculation based on conditions
   - Mock Thrift client for controller tests
   - Test connection pooling and retry logic
   - Validate topology serialization

2. **Integration Tests**
   - Test with real Storm cluster
   - Validate end-to-end topology submission
   - Test failure scenarios

3. **Performance Tests**
   - Benchmark JAR upload performance
   - Test connection pool under load
   - Measure controller reconciliation performance

#### 5.2 Migration Steps

1. **Feature Flag Implementation**
   ```go
   type ControllerConfig struct {
       UseThriftClient bool `json:"useThriftClient"`
       // Allows gradual rollout
   }
   ```

2. **Parallel Run**
   - Run both CLI and Thrift implementations
   - Compare results and performance
   - Identify and fix discrepancies

3. **Gradual Rollout**
   - Enable Thrift for read operations first
   - Move write operations incrementally
   - Monitor metrics and errors

### Phase 6: Cleanup and Documentation (Week 9)

#### 6.1 Remove Legacy Code
- Remove CLI-based implementation
- Remove REST client code
- Update Dockerfile to remove Storm CLI

#### 6.2 Update Documentation
- Update CLAUDE.md with new architecture
- Create operation guide for Thrift client
- Document configuration options
- Add troubleshooting guide

## Configuration Schema

### Thrift Client Configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: storm-controller-config
data:
  thrift-config.yaml: |
    client:
      maxConnections: 10
      connectTimeout: 30s
      requestTimeout: 60s
      idleTimeout: 5m
      retryPolicy:
        maxRetries: 3
        backoffMultiplier: 2
        initialBackoff: 1s
        maxBackoff: 30s
    
    jarCache:
      enabled: true
      maxSize: 10Gi
      ttl: 24h
      directory: /var/cache/storm-jars
```

### Controller Feature Flags
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: storm-controller-features
data:
  features.yaml: |
    thriftClient:
      enabled: true
      rolloutPercentage: 100
    
    clusterProvisioning:
      enabled: true
      useHelm: false  # Use native K8s resources
    
    workerPoolAutoscaling:
      enabled: true
      defaultPolicy: "topology-based"
    
    admissionWebhooks:
      enabled: true
      failurePolicy: "Fail"
```

## Success Metrics

1. **Performance Improvements**
   - Topology submission time reduced by 50%
   - JAR upload reliability improved to 99.9%
   - Controller reconciliation latency < 100ms

2. **Operational Improvements**
   - Zero dependency on Storm CLI
   - Automated cluster provisioning
   - Working autoscaling for worker pools

3. **Developer Experience**
   - Clear error messages from Thrift exceptions
   - Comprehensive metrics and observability
   - Well-documented APIs and configurations

4. **Code Quality**
   - 90%+ unit test coverage for business logic (state machines)
   - All state transitions documented and tested
   - Zero dependency on K8s for business logic tests
   - Fast test execution (< 1 second for state machine tests)

## Risks and Mitigations

1. **Thrift Protocol Changes**
   - Risk: Storm updates may change Thrift interface
   - Mitigation: Version detection and compatibility layer

2. **Connection Management**
   - Risk: Connection leaks or exhaustion
   - Mitigation: Robust pooling with health checks

3. **Large JAR Uploads**
   - Risk: Memory exhaustion or timeouts
   - Mitigation: Streaming uploads with progress tracking

4. **Backward Compatibility**
   - Risk: Breaking existing deployments
   - Mitigation: Feature flags and gradual rollout

## Timeline Summary

- **Weeks 1-2**: Thrift infrastructure setup
- **Week 3**: Controller enhancements
- **Week 4**: Implement testable state machines
- **Week 5**: Coordination layer
- **Week 6**: JAR management
- **Weeks 7-8**: Testing and migration
- **Week 9**: Cleanup and documentation

Total estimated time: 9 weeks with 2 developers

## Conclusion

This plan provides a comprehensive approach to enhancing the Storm controller architecture while migrating to a native Thrift client. The phased approach allows for incremental delivery of value while maintaining system stability. The enhanced architecture will provide better reliability, performance, and features compared to the current CLI-based implementation.