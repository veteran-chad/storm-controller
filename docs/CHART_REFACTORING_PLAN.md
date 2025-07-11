# Storm Kubernetes Chart Refactoring Plan

## Overview

Refactor the current monolithic Storm Helm chart into four separate charts for better modularity and deployment flexibility.

## Chart Architecture

### 1. **storm-kubernetes** (Storm Cluster Only)
- Pure Storm cluster deployment (Nimbus, Supervisor, UI)
- No CRDs or controller
- No Zookeeper (expects external)
- Use case: Traditional Helm-based Storm deployments

### 2. **storm-shared** (Library Chart)
- Shared templates and helpers
- Common labels, annotations, and naming functions
- Global settings definitions
- Image configuration templates
- Security context templates

### 3. **storm-operator** (Operator & CRDs)
- Storm CRDs (StormCluster, StormTopology, StormWorkerPool)
- Storm controller deployment
- Default Zookeeper deployment (Bitnami subchart)
- Controller ConfigMap with defaults
- Use case: Install operator once per cluster

### 4. **storm-crd-cluster** (CRD-based Cluster)
- Creates StormCluster CRD resource
- No actual Storm deployments (operator handles that)
- Configuration for existing or default Zookeeper
- Use case: GitOps-friendly cluster definitions

## Implementation Plan

### Phase 1: Create storm-shared Library Chart

```yaml
# charts/storm-shared/Chart.yaml
apiVersion: v2
name: storm-shared
description: Shared library for Storm Kubernetes charts
type: library
version: 0.1.0
```

Key templates:
- `_helpers.tpl`: Common naming functions
- `_images.tpl`: Image resolution with global overrides
- `_labels.tpl`: Standard labels
- `_security.tpl`: Security contexts
- `_config.tpl`: Storm configuration helpers

### Phase 2: Create storm-operator Chart

Structure:
```
charts/storm-operator/
├── Chart.yaml
├── values.yaml
├── crds/                    # CRD definitions
├── templates/
│   ├── controller/
│   │   ├── deployment.yaml
│   │   ├── configmap.yaml  # Controller defaults
│   │   ├── rbac.yaml
│   │   └── service.yaml
│   └── NOTES.txt
└── charts/
    └── zookeeper/          # Bitnami dependency
```

Controller ConfigMap structure:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: storm-operator-config
data:
  defaults.yaml: |
    # Default Storm configuration
    storm:
      image:
        registry: {{ .Values.global.imageRegistry | default "docker.io" }}
        repository: {{ .Values.defaults.storm.image.repository }}
        tag: {{ .Values.defaults.storm.image.tag }}
      config:
        nimbus.seeds: ["nimbus"]
        storm.zookeeper.servers: {{ .Values.defaults.zookeeper.servers }}
        storm.zookeeper.root: "/storm/{{ .ClusterName }}"  # Isolation by cluster
        storm.local.dir: "/storm/data"
    # Default cluster sizing
    cluster:
      nimbus:
        replicas: {{ .Values.defaults.cluster.nimbus.replicas }}
      supervisor:
        replicas: {{ .Values.defaults.cluster.supervisor.replicas }}
        slotsPerSupervisor: {{ .Values.defaults.cluster.supervisor.slots }}
```

### Phase 3: Refactor storm-kubernetes Chart

Remove:
- CRD installation
- Controller deployment
- Zookeeper subchart

Add:
- Dependency on storm-shared
- External Zookeeper configuration

### Phase 4: Create storm-crd-cluster Chart

```yaml
# charts/storm-crd-cluster/Chart.yaml
apiVersion: v2
name: storm-crd-cluster
description: Deploy Storm cluster using CRDs
type: application
version: 0.1.0
dependencies:
  - name: storm-shared
    version: "0.1.0"
    repository: "file://../storm-shared"
```

Templates:
```yaml
# templates/stormcluster.yaml
apiVersion: storm.apache.org/v1beta1
kind: StormCluster
metadata:
  name: {{ include "storm-shared.fullname" . }}
spec:
  clusterName: {{ include "storm-shared.fullname" . }}
  {{- if .Values.zookeeper.external.enabled }}
  zookeeper:
    servers: {{ .Values.zookeeper.external.servers }}
    root: {{ .Values.zookeeper.external.root | default (printf "/storm/%s" (include "storm-shared.fullname" .)) }}
  {{- else }}
  zookeeper:
    servers: {{ .Values.zookeeper.default.servers }}
    root: {{ printf "/storm/%s" (include "storm-shared.fullname" .) }}
  {{- end }}
  image:
    registry: {{ include "storm-shared.images.registry" . }}
    repository: {{ include "storm-shared.images.repository" (dict "imageRoot" .Values.storm.image "global" .Values.global "defaultRepository" "storm") }}
    tag: {{ .Values.storm.image.tag }}
  nimbus:
    replicas: {{ .Values.nimbus.replicas }}
    {{- with .Values.nimbus.resources }}
    resources: {{- toYaml . | nindent 6 }}
    {{- end }}
  supervisor:
    replicas: {{ .Values.supervisor.replicas }}
    slotsPerSupervisor: {{ .Values.supervisor.slotsPerSupervisor }}
    {{- with .Values.supervisor.resources }}
    resources: {{- toYaml . | nindent 6 }}
    {{- end }}
  ui:
    enabled: {{ .Values.ui.enabled }}
    {{- with .Values.ui.resources }}
    resources: {{- toYaml . | nindent 6 }}
    {{- end }}
  config: {{- toYaml .Values.storm.config | nindent 4 }}
```

## Controller Code Changes

### 1. ConfigMap Integration

```go
// pkg/controller/config.go
type ControllerConfig struct {
    Defaults DefaultConfig `yaml:"defaults"`
}

type DefaultConfig struct {
    Storm   StormDefaults   `yaml:"storm"`
    Cluster ClusterDefaults `yaml:"cluster"`
}

func (r *StormClusterReconciler) loadControllerConfig() (*ControllerConfig, error) {
    cm := &corev1.ConfigMap{}
    err := r.Get(ctx, types.NamespacedName{
        Name:      "storm-operator-config",
        Namespace: r.OperatorNamespace,
    }, cm)
    if err != nil {
        return nil, err
    }
    
    config := &ControllerConfig{}
    if err := yaml.Unmarshal([]byte(cm.Data["defaults.yaml"]), config); err != nil {
        return nil, err
    }
    return config, nil
}
```

### 2. Config Merging

```go
// Merge configs: Controller defaults -> CRD spec -> Final config
func (r *StormClusterReconciler) mergeConfigs(
    defaults map[string]interface{},
    crdConfig map[string]interface{},
) map[string]interface{} {
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
    clusterName := merged["cluster.name"].(string)
    if _, exists := merged["storm.zookeeper.root"]; !exists {
        merged["storm.zookeeper.root"] = fmt.Sprintf("/storm/%s", clusterName)
    }
    
    return merged
}
```

### 3. Zookeeper Path Isolation

```go
// Ensure each cluster uses isolated Zookeeper path
func (r *StormClusterReconciler) getZookeeperRoot(cluster *stormv1beta1.StormCluster) string {
    if cluster.Spec.Zookeeper.Root != "" {
        return cluster.Spec.Zookeeper.Root
    }
    return fmt.Sprintf("/storm/%s", cluster.Name)
}
```

## Global Settings Pattern

All charts will support:

```yaml
global:
  imageRegistry: "my-registry.com"
  imagePullSecrets:
    - name: my-registry-secret
  storageClass: "fast-ssd"
```

Implementation in storm-shared:
```yaml
{{/*
Get the image registry
*/}}
{{- define "storm-shared.images.registry" -}}
{{- $registry := .Values.storm.image.registry -}}
{{- if .Values.global.imageRegistry -}}
  {{- $registry = .Values.global.imageRegistry -}}
{{- end -}}
{{- $registry -}}
{{- end -}}
```

## Testing Strategy

### 1. Unit Tests
- Controller config loading and merging
- Zookeeper path generation
- Template rendering for each chart

### 2. Integration Tests
- Deploy storm-operator with default Zookeeper
- Create StormCluster CRD
- Verify cluster creation with isolated Zookeeper paths
- Test with external Zookeeper

### 3. E2E Test Scenarios

```bash
# Scenario 1: Operator with default Zookeeper
helm install storm-operator ./charts/storm-operator

# Scenario 2: Multiple clusters with isolation
helm install cluster-1 ./charts/storm-crd-cluster --set clusterName=prod
helm install cluster-2 ./charts/storm-crd-cluster --set clusterName=staging

# Verify Zookeeper isolation
kubectl exec -it zookeeper-0 -- zkCli.sh ls /storm
# Should show: [prod, staging]

# Scenario 3: External Zookeeper
helm install cluster-3 ./charts/storm-crd-cluster \
  --set zookeeper.external.enabled=true \
  --set zookeeper.external.servers="{zk1:2181,zk2:2181}"
```

### 4. Upgrade Testing
- Test upgrading operator without affecting running clusters
- Test updating cluster CRDs
- Verify config changes propagate correctly

## Benefits

1. **Separation of Concerns**: Operator lifecycle independent of clusters
2. **Multi-tenancy**: Multiple Storm clusters with Zookeeper isolation
3. **GitOps Ready**: Declarative cluster definitions via CRDs
4. **Flexibility**: Mix traditional and CRD-based deployments
5. **Maintainability**: Shared library reduces duplication

## Implementation Order

1. Create storm-shared library chart
2. Create storm-operator chart with controller changes
3. Create storm-crd-cluster chart
4. Refactor storm-kubernetes to use shared library
5. Update controller code for config merging
6. Implement comprehensive testing
7. Update documentation