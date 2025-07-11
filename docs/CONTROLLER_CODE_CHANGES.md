# Controller Code Changes for Multi-Chart Architecture

## Overview

The controller needs to be updated to support the new multi-chart architecture with proper config merging and Zookeeper path isolation.

## Required Changes

### 1. Config Loading from ConfigMap

**File: `pkg/config/loader.go`** (new)

```go
package config

import (
    "context"
    "fmt"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/yaml"
)

type OperatorConfig struct {
    Defaults DefaultsConfig `yaml:"defaults"`
}

type DefaultsConfig struct {
    Storm     StormDefaults     `yaml:"storm"`
    Cluster   ClusterDefaults   `yaml:"cluster"`
    Zookeeper ZookeeperDefaults `yaml:"zookeeper"`
}

type StormDefaults struct {
    Image  ImageDefaults         `yaml:"image"`
    Config map[string]interface{} `yaml:"config"`
}

type ImageDefaults struct {
    Registry   string `yaml:"registry"`
    Repository string `yaml:"repository"`
    Tag        string `yaml:"tag"`
}

type ClusterDefaults struct {
    Nimbus     NimbusDefaults     `yaml:"nimbus"`
    Supervisor SupervisorDefaults `yaml:"supervisor"`
    UI         UIDefaults         `yaml:"ui"`
}

type NimbusDefaults struct {
    Replicas int32 `yaml:"replicas"`
}

type SupervisorDefaults struct {
    Replicas           int32 `yaml:"replicas"`
    SlotsPerSupervisor int32 `yaml:"slotsPerSupervisor"`
}

type UIDefaults struct {
    Enabled bool `yaml:"enabled"`
}

type ZookeeperDefaults struct {
    Servers           []string `yaml:"servers"`
    ConnectionTimeout int      `yaml:"connectionTimeout"`
    SessionTimeout    int      `yaml:"sessionTimeout"`
}

func LoadOperatorConfig(ctx context.Context, c client.Client, namespace string) (*OperatorConfig, error) {
    cm := &corev1.ConfigMap{}
    err := c.Get(ctx, types.NamespacedName{
        Name:      "storm-operator-config",
        Namespace: namespace,
    }, cm)
    if err != nil {
        return nil, fmt.Errorf("failed to get operator config: %w", err)
    }
    
    configData, ok := cm.Data["defaults.yaml"]
    if !ok {
        return nil, fmt.Errorf("defaults.yaml not found in configmap")
    }
    
    config := &OperatorConfig{}
    if err := yaml.Unmarshal([]byte(configData), config); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }
    
    return config, nil
}
```

### 2. Config Merger

**File: `pkg/config/merger.go`** (new)

```go
package config

import (
    "fmt"
    stormv1beta1 "github.com/apache/storm-operator/api/v1beta1"
)

// MergeStormConfig merges operator defaults with CRD spec config
func MergeStormConfig(defaults map[string]interface{}, crdConfig map[string]interface{}, clusterName string) map[string]interface{} {
    merged := make(map[string]interface{})
    
    // Start with defaults
    for k, v := range defaults {
        merged[k] = v
    }
    
    // Override with CRD config
    for k, v := range crdConfig {
        merged[k] = v
    }
    
    // Ensure cluster-specific Zookeeper root
    if _, exists := merged["storm.zookeeper.root"]; !exists {
        merged["storm.zookeeper.root"] = fmt.Sprintf("/storm/%s", clusterName)
    }
    
    return merged
}

// ApplyDefaults applies operator defaults to StormCluster if not specified
func ApplyDefaults(cluster *stormv1beta1.StormCluster, defaults *OperatorConfig) {
    // Apply image defaults if not specified
    if cluster.Spec.Image.Registry == "" && defaults.Storm.Image.Registry != "" {
        cluster.Spec.Image.Registry = defaults.Storm.Image.Registry
    }
    if cluster.Spec.Image.Repository == "" && defaults.Storm.Image.Repository != "" {
        cluster.Spec.Image.Repository = defaults.Storm.Image.Repository
    }
    if cluster.Spec.Image.Tag == "" && defaults.Storm.Image.Tag != "" {
        cluster.Spec.Image.Tag = defaults.Storm.Image.Tag
    }
    
    // Apply cluster sizing defaults
    if cluster.Spec.Nimbus.Replicas == 0 {
        cluster.Spec.Nimbus.Replicas = defaults.Cluster.Nimbus.Replicas
    }
    if cluster.Spec.Supervisor.Replicas == 0 {
        cluster.Spec.Supervisor.Replicas = defaults.Cluster.Supervisor.Replicas
    }
    if cluster.Spec.Supervisor.SlotsPerSupervisor == 0 {
        cluster.Spec.Supervisor.SlotsPerSupervisor = defaults.Cluster.Supervisor.SlotsPerSupervisor
    }
    
    // Apply Zookeeper defaults if not specified
    if len(cluster.Spec.Zookeeper.Servers) == 0 && len(defaults.Zookeeper.Servers) > 0 {
        cluster.Spec.Zookeeper.Servers = defaults.Zookeeper.Servers
    }
    
    // Ensure Zookeeper root path isolation
    if cluster.Spec.Zookeeper.Root == "" {
        cluster.Spec.Zookeeper.Root = fmt.Sprintf("/storm/%s", cluster.Name)
    }
}
```

### 3. Update StormCluster Controller

**File: `controllers/stormcluster_controller.go`** (updates)

```go
import (
    "github.com/apache/storm-operator/pkg/config"
)

type StormClusterReconciler struct {
    client.Client
    Scheme           *runtime.Scheme
    OperatorConfig   *config.OperatorConfig
    OperatorNamespace string
}

func (r *StormClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    // Load operator config if not already loaded
    if r.OperatorConfig == nil {
        cfg, err := config.LoadOperatorConfig(ctx, r.Client, r.OperatorNamespace)
        if err != nil {
            log.Error(err, "Failed to load operator config")
            return ctrl.Result{}, err
        }
        r.OperatorConfig = cfg
    }
    
    // Fetch the StormCluster instance
    cluster := &stormv1beta1.StormCluster{}
    err := r.Get(ctx, req.NamespacedName, cluster)
    if err != nil {
        if errors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }
    
    // Apply defaults from operator config
    config.ApplyDefaults(cluster, r.OperatorConfig)
    
    // Continue with existing reconciliation logic...
}

// When creating ConfigMap for Storm
func (r *StormClusterReconciler) createStormConfigMap(cluster *stormv1beta1.StormCluster) *corev1.ConfigMap {
    // Merge operator defaults with cluster config
    mergedConfig := config.MergeStormConfig(
        r.OperatorConfig.Defaults.Storm.Config,
        cluster.Spec.Config,
        cluster.Name,
    )
    
    // Convert to YAML for storm.yaml
    stormYaml, _ := yaml.Marshal(mergedConfig)
    
    return &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("%s-storm-config", cluster.Name),
            Namespace: cluster.Namespace,
        },
        Data: map[string]string{
            "storm.yaml": string(stormYaml),
        },
    }
}
```

### 4. Update Main to Pass Operator Namespace

**File: `main.go`** (updates)

```go
func main() {
    var operatorNamespace string
    flag.StringVar(&operatorNamespace, "operator-namespace", os.Getenv("OPERATOR_NAMESPACE"), 
        "The namespace where the operator is deployed")
    
    // ... existing code ...
    
    if err = (&controllers.StormClusterReconciler{
        Client:            mgr.GetClient(),
        Scheme:            mgr.GetScheme(),
        OperatorNamespace: operatorNamespace,
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "StormCluster")
        os.Exit(1)
    }
}
```

### 5. Update API Types for New Fields

**File: `api/v1beta1/stormcluster_types.go`** (updates)

```go
// Add Zookeeper root field
type ZookeeperSpec struct {
    // Servers is the list of Zookeeper servers
    Servers []string `json:"servers"`
    
    // Root is the Zookeeper root path for this cluster
    // +optional
    Root string `json:"root,omitempty"`
    
    // ConnectionTimeout in milliseconds
    // +optional
    ConnectionTimeout int `json:"connectionTimeout,omitempty"`
    
    // SessionTimeout in milliseconds
    // +optional
    SessionTimeout int `json:"sessionTimeout,omitempty"`
}

// Add node selector and tolerations to component specs
type NimbusSpec struct {
    // ... existing fields ...
    
    // NodeSelector for pod assignment
    // +optional
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`
    
    // Tolerations for pod assignment
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// Similar additions for SupervisorSpec and UISpec
```

### 6. Testing Updates

**File: `controllers/stormcluster_controller_test.go`** (new tests)

```go
func TestConfigMerging(t *testing.T) {
    tests := []struct {
        name         string
        defaults     map[string]interface{}
        crdConfig    map[string]interface{}
        clusterName  string
        expectedRoot string
    }{
        {
            name: "default zookeeper root",
            defaults: map[string]interface{}{
                "nimbus.seeds": []string{"nimbus"},
            },
            crdConfig:    map[string]interface{}{},
            clusterName:  "test-cluster",
            expectedRoot: "/storm/test-cluster",
        },
        {
            name: "custom zookeeper root",
            defaults: map[string]interface{}{},
            crdConfig: map[string]interface{}{
                "storm.zookeeper.root": "/custom/path",
            },
            clusterName:  "test-cluster",
            expectedRoot: "/custom/path",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            merged := config.MergeStormConfig(tt.defaults, tt.crdConfig, tt.clusterName)
            root, _ := merged["storm.zookeeper.root"].(string)
            assert.Equal(t, tt.expectedRoot, root)
        })
    }
}
```

## Implementation Order

1. Create config package with loader and merger
2. Update API types to include new fields
3. Run `make generate` to update DeepCopy methods
4. Update StormCluster controller to use config
5. Update main.go to pass operator namespace
6. Add comprehensive tests
7. Update existing tests to work with new structure

## Benefits

- **Config Management**: Centralized defaults in operator ConfigMap
- **Multi-tenancy**: Each cluster gets isolated Zookeeper path
- **Flexibility**: Override any default at the cluster level
- **GitOps**: All config in declarative resources