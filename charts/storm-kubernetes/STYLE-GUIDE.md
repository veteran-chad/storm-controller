# Storm Kubernetes Helm Chart Style Guide

This guide outlines the patterns and conventions for using the Bitnami common library in the Storm Kubernetes Helm chart. These patterns are derived from analyzing Bitnami's Zookeeper chart and the common library itself, representing best practices for creating consistent, maintainable Helm charts.

## Important: Image Security Configuration

**CRITICAL**: The Bitnami common library includes strict image security validation that will **fail deployments by default** if non-Bitnami images are used. Since Storm images are not stored in Bitnami repositories, you MUST configure the following:

```yaml
# In values.yaml
global:
  security:
    allowInsecureImages: true  # Required for non-Bitnami images
```

Without this setting, the chart will fail with an error about insecure images. This is a security feature of the Bitnami common library that validates only original Bitnami/VMware Tanzu Application Catalog images are used.

## 1. Name Resolution

Always use common library functions for generating names to ensure consistency:

```yaml
# Full name generation (includes release name)
name: {{ include "common.names.fullname" . }}

# Chart name only
chart: {{ include "common.names.chart" . }}

# For referencing dependencies
{{ include "common.names.dependency.fullname" (dict "chartName" "zookeeper" "chartValues" .Values.zookeeper "context" $) }}
```

## 2. Labels and Selectors

### Standard Labels
```yaml
metadata:
  labels:
    {{- include "common.labels.standard" ( dict "customLabels" .Values.commonLabels "context" $ ) | nindent 4 }}
```

### Match Labels for Selectors
```yaml
spec:
  selector:
    matchLabels:
      {{- include "common.labels.matchLabels" ( dict "customLabels" $podLabels "context" $ ) | nindent 6 }}
```

### Merging Pod Labels
```yaml
{{- $podLabels := include "common.tplvalues.merge" ( dict "values" ( list .Values.podLabels .Values.commonLabels ) "context" . ) }}
```

## 3. ConfigMap and Value Merging

### Template Value Rendering
```yaml
# Render a value that might contain template syntax
{{- include "common.tplvalues.render" ( dict "value" .Values.configuration "context" $ ) | nindent 4 }}
```

### Merging Multiple Value Sources
```yaml
# Merge annotations from multiple sources
{{- $annotations := include "common.tplvalues.merge" ( dict "values" ( list .Values.service.annotations .Values.commonAnnotations ) "context" . ) }}
```

## 4. Secret Management

### Password Generation/Management
```yaml
# Automatically generate passwords if not provided
{{ include "common.secrets.passwords.manage" (dict "secret" "storm-auth" "key" "password" "providedValues" (list "auth.password") "context" $) }}
```

### Secret Name Resolution
```yaml
# Get secret name (existing or generated)
secretName: {{ include "common.secrets.name" (dict "existingSecret" .Values.auth.existingSecret "defaultNameSuffix" "auth" "context" $) }}
```

### Secret Key Mapping
```yaml
# Get the correct key from a secret
- name: STORM_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "common.secrets.name" (dict "existingSecret" .Values.auth.existingSecret "defaultNameSuffix" "auth" "context" $) }}
      key: {{ include "common.secrets.key" (dict "existingSecret" .Values.auth.existingSecret "key" "password") }}
```

## 5. Image Configuration

### Image Name Construction
```yaml
image: {{ include "common.images.image" (dict "imageRoot" .Values.nimbus.image "global" .Values.global) }}
```

### Image Pull Secrets
```yaml
imagePullSecrets:
  {{- include "common.images.renderPullSecrets" ( dict "images" (list .Values.nimbus.image .Values.supervisor.image .Values.ui.image) "context" $) }}
```

## 6. Security Context

### Pod Security Context
```yaml
spec:
  securityContext:
    {{- include "common.compatibility.renderSecurityContext" (dict "secContext" .Values.podSecurityContext "context" $) | nindent 8 }}
```

### Container Security Context
```yaml
containers:
  - name: nimbus
    securityContext:
      {{- include "common.compatibility.renderSecurityContext" (dict "secContext" .Values.nimbus.containerSecurityContext "context" $) | nindent 12 }}
```

## 7. Service Account

Create a helper function for service account names:
```yaml
{{- define "storm.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ include "common.names.fullname" . }}
{{- else -}}
    {{ .Values.serviceAccount.name | default "default" }}
{{- end -}}
{{- end -}}
```

## 8. Resource Management

### Using Resource Presets
```yaml
resources:
  {{- if .Values.nimbus.resources }}
  {{- toYaml .Values.nimbus.resources | nindent 12 }}
  {{- else if ne .Values.nimbus.resourcesPreset "none" }}
  {{- include "common.resources.preset" (dict "type" .Values.nimbus.resourcesPreset) | nindent 12 }}
  {{- end }}
```

## 9. Affinity Configuration

### Pod Affinity/Anti-Affinity
```yaml
affinity:
  podAffinity:
    {{- include "common.affinities.pods" (dict "type" .Values.podAffinityPreset "component" "nimbus" "customLabels" $podLabels "context" $) | nindent 10 }}
  podAntiAffinity:
    {{- include "common.affinities.pods" (dict "type" .Values.podAntiAffinityPreset "component" "nimbus" "customLabels" $podLabels "context" $) | nindent 10 }}
  nodeAffinity:
    {{- include "common.affinities.nodes" (dict "type" .Values.nodeAffinityPreset.type "key" .Values.nodeAffinityPreset.key "values" .Values.nodeAffinityPreset.values) | nindent 10 }}
```

## 10. API Version and Capabilities

```yaml
apiVersion: {{ include "common.capabilities.statefulset.apiVersion" . }}
```

## 11. Warnings and Validations

### Rolling Tag Warning
```yaml
{{- include "common.warnings.rollingTag" .Values.nimbus.image }}
```

### Custom Validation
```yaml
{{- if and .Values.auth.enabled (not .Values.auth.password) (not .Values.auth.existingSecret) }}
  {{- fail "Storm authentication requires either auth.password or auth.existingSecret" }}
{{- end }}
```

## 12. Best Practices

1. **Always use `include` instead of `template`** when you need the output
2. **Use `dict` to pass parameters** to common templates
3. **Use `nindent` for proper YAML indentation**
4. **Merge custom labels/annotations** with defaults using `common.tplvalues.merge`
5. **Support both direct values and presets** for resources
6. **Add conditional blocks** for optional features (TLS, auth, metrics)
7. **Use checksum annotations** to trigger pod restarts on config changes:
   ```yaml
   annotations:
     checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
   ```

## 13. Common Values Structure

Organize values.yaml to follow Bitnami patterns:

```yaml
global:
  imageRegistry: ""
  imagePullSecrets: []
  storageClass: ""

commonLabels: {}
commonAnnotations: {}

image:
  registry: docker.io
  repository: apache/storm
  tag: 2.8.1
  digest: ""
  pullPolicy: IfNotPresent
  pullSecrets: []

serviceAccount:
  create: true
  name: ""
  annotations: {}
  automountServiceAccountToken: false

podSecurityContext:
  enabled: true
  fsGroup: 1001
  fsGroupChangePolicy: Always

containerSecurityContext:
  enabled: true
  runAsUser: 1001
  runAsNonRoot: true
  privileged: false
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: "RuntimeDefault"

resourcesPreset: "nano"
resources: {}

nodeAffinityPreset:
  type: ""
  key: ""
  values: []

podAffinityPreset: ""
podAntiAffinityPreset: soft
```

## 14. Service Definitions

Use common patterns for services:

```yaml
service:
  type: ClusterIP
  ports:
    thrift: 6627
  annotations: {}
  loadBalancerIP: ""
  loadBalancerSourceRanges: []
  externalTrafficPolicy: Cluster
  sessionAffinity: None
  sessionAffinityConfig: {}
```

## 15. Persistence Configuration

Standard persistence configuration:

```yaml
persistence:
  enabled: true
  storageClass: ""
  accessModes:
    - ReadWriteOnce
  size: 8Gi
  annotations: {}
  existingClaim: ""
  selector: {}
```

## 16. Common Library Advanced Features

Based on analysis of the Bitnami common library v2.31.3:

### Available Template Functions

The common library provides these key function categories:

1. **Naming Functions** (`common.names.*`):
   - `name`: Chart name with optional overrides
   - `fullname`: Release-prefixed name with truncation
   - `namespace`: Namespace resolution with overrides
   - `chart`: Chart name with version
   - `dependency.fullname`: Reference dependency resources

2. **Image Functions** (`common.images.*`):
   - `image`: Construct full image reference with registry/tag/digest
   - `renderPullSecrets`: Process and render pull secrets with template support
   - `version`: Extract semantic version from tags

3. **Secret Functions** (`common.secrets.*`):
   - `passwords.manage`: Sophisticated password generation with upgrade support
   - `name`: Handle existing vs generated secret names
   - `key`: Map secret keys with defaults
   - `lookup`: Retrieve existing secret values
   - `exists`: Check secret existence

4. **Validation Functions** (`common.validations.*`):
   - `values.single.empty`: Validate single required value
   - `values.multiple.empty`: Validate multiple required values
   - Support for password validation in upgrade scenarios

5. **Template Value Functions** (`common.tplvalues.*`):
   - `render`: Render values containing Go templates
   - `merge`: Deep merge multiple value sources

6. **Capability Detection** (`common.capabilities.*`):
   - `kubeVersion`: Get target Kubernetes version
   - API version helpers for all resource types
   - `apiVersions.has`: Check API availability

7. **Utility Functions** (`common.utils.*`):
   - `getValueFromKey`: Safely get nested values
   - `checksumTemplate`: Generate config checksums
   - `fieldToEnvVar`: Convert to environment variable format

### Key Considerations

1. **Minimum Requirements**:
   - Helm 3.8.0+
   - Kubernetes 1.23+
   - Uses Helm's built-in functions extensively

2. **Security Context Compatibility**:
   - `common.compatibility.renderSecurityContext`: Adapts security contexts for different platforms
   - Special handling for OpenShift deployments

3. **Error Handling**:
   - Functions include comprehensive error messages
   - Validation failures provide clear guidance
   - Support for warnings without failing deployment

4. **Template Evaluation**:
   - Many functions support nested template evaluation
   - Use `$` context for global scope access
   - Proper quote handling for complex values

### Usage Tips

1. **Always pass context**: Most functions require `"context" $` parameter
2. **Use dict for parameters**: All functions expect parameters via `dict`
3. **Check function outputs**: Some functions return empty strings on error
4. **Test with different values**: Validate behavior with empty/nil values
5. **Review warnings**: Even if deployment succeeds, check for warnings

### Rolling Tag Warning Example

```yaml
# Add to deployment templates to warn about mutable tags
{{- include "common.warnings.rollingTag" .Values.nimbus.image }}
{{- include "common.warnings.rollingTag" .Values.supervisor.image }}
```

This will warn users if they're using tags like `latest` that can change.

## 17. Configuration Management Pattern

For charts with multiple configuration sections (e.g., Zookeeper, UI, Nimbus), use a modular approach with the `extraConfig` pattern:

### The extraConfig Pattern

Each component section should follow this structure:

```yaml
componentName:
  enabled: true/false              # Toggle component on/off
  # ... other component-specific settings ...
  extraConfig: {}                  # Additional configuration overrides
```

### Structure in values.yaml

```yaml
## @section Zookeeper parameters
## @param zookeeper.enabled Deploy Zookeeper as a dependency
zookeeper:
  enabled: true
  ## External Zookeeper configuration
  ## @param zookeeper.external.servers List of external Zookeeper servers (when enabled=false)
  external:
    servers: []
  ## @param zookeeper.extraConfig Extra configuration for Zookeeper
  ## eg.:
  ## extraConfig:
  ##   storm.zookeeper.port: 2181
  ##   storm.zookeeper.root: "/storm"
  ##   storm.zookeeper.session.timeout: 20000
  extraConfig: {}

## @section Storm UI parameters
## @param ui.enabled Deploy Storm UI
ui:
  enabled: true
  ## @param ui.extraConfig Extra configuration for Storm UI
  ## eg.:
  ## extraConfig:
  ##   ui.port: 8080
  ##   ui.title: "Storm UI"
  ##   ui.childopts: "-Xmx768m"
  extraConfig: {}

## @section Storm Cluster Configuration
## Complete Storm cluster configuration with all available options and defaults
clusterConfig:
  # All Storm configuration options with defaults
  # These are overridden by component-specific extraConfig values
```

### extraConfig Guidelines

1. **Always use empty object as default**: `extraConfig: {}`
2. **Document with examples**: Show common configuration options in comments
3. **List all available options**: Include all possible configuration keys in the example
4. **Use proper indentation**: Examples should show correct YAML structure
5. **Reference defaults**: Point users to where default values come from

### Configuration Precedence

Component-specific configurations take precedence over general cluster configuration:
1. **Component extraConfig** (highest priority) - e.g., `ui.extraConfig`
2. **General clusterConfig** (lowest priority)

### Helper Template Pattern

Create a helper template to handle configuration rendering with proper precedence:

```yaml
{{/*
Render cluster configuration with proper precedence
This helper filters out configuration keys that are managed by specific sections
to avoid duplication. Section-specific configs take precedence over clusterConfig.
*/}}
{{- define "storm.renderClusterConfig" -}}
{{- if .Values.clusterConfig }}
{{- range $key, $value := .Values.clusterConfig }}
{{- if ne $key "storm.zookeeper.servers" }}
{{- if not (and $.Values.zookeeper.extraConfig (hasKey $.Values.zookeeper.extraConfig $key)) }}
{{- if not (and $.Values.ui.enabled $.Values.ui.extraConfig (hasKey $.Values.ui.extraConfig $key)) }}
{{ $key }}: {{ $value | toJson }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
```

### ConfigMap Template Pattern

Use the helper in your ConfigMap template:

```yaml
data:
  storm.yaml: |
    # Component-specific settings
    {{- if .Values.zookeeper.enabled }}
    storm.zookeeper.servers:
      - {{ include "storm.zookeeperHeadlessService" . }}.{{ include "common.names.namespace" . }}.svc.{{ .Values.clusterDomain }}
    {{- end }}
    
    # Additional component configuration
    {{- if .Values.zookeeper.extraConfig }}
    {{- range $key, $value := .Values.zookeeper.extraConfig }}
    {{ $key }}: {{ $value | toJson }}
    {{- end }}
    {{- end }}
    
    # UI configuration (when enabled)
    {{- if .Values.ui.enabled }}
    {{- if .Values.ui.extraConfig }}
    {{- range $key, $value := .Values.ui.extraConfig }}
    {{ $key }}: {{ $value | toJson }}
    {{- end }}
    {{- end }}
    {{- end }}
    
    # General cluster configuration (filtered)
    {{- include "storm.renderClusterConfig" . | nindent 4 }}
```

### Benefits of This Pattern

1. **Clear separation of concerns** - Component configs are isolated
2. **Flexible overrides** - Users can override specific component settings
3. **No duplication** - Helper ensures configs aren't duplicated
4. **Easy to extend** - Add new components by extending the helper
5. **Maintainable** - Logic is centralized in the helper template

### Implementing extraConfig in Templates

When rendering extraConfig values in templates:

```yaml
# Render component's extraConfig when component is enabled
{{- if .Values.componentName.enabled }}
{{- if .Values.componentName.extraConfig }}
{{- range $key, $value := .Values.componentName.extraConfig }}
{{ $key }}: {{ $value | toJson }}
{{- end }}
{{- end }}
{{- end }}
```

### Adding a New Component with extraConfig

To add a new component following this pattern:

1. **Add to values.yaml**:
```yaml
## @section Component Name parameters
## @param componentName.enabled Deploy Component Name
componentName:
  enabled: true
  ## @param componentName.extraConfig Extra configuration for Component Name
  ## eg.:
  ## extraConfig:
  ##   component.setting1: value1
  ##   component.setting2: value2
  extraConfig: {}
```

2. **Update the helper template** (`_helpers.tpl`):
```yaml
{{- if not (and $.Values.componentName.enabled $.Values.componentName.extraConfig (hasKey $.Values.componentName.extraConfig $key)) }}
```

3. **Add to ConfigMap template**:
```yaml
# Component Name configuration
{{- if .Values.componentName.enabled }}
{{- if .Values.componentName.extraConfig }}
{{- range $key, $value := .Values.componentName.extraConfig }}
{{ $key }}: {{ $value | toJson }}
{{- end }}
{{- end }}
{{- end }}
```

### Example: Complete Component Configuration

Here's a complete example for a hypothetical Nimbus component:

```yaml
# In values.yaml
nimbus:
  enabled: true
  replicaCount: 1
  resources:
    limits:
      memory: 1Gi
      cpu: 1000m
  extraConfig:
    nimbus.thrift.port: 6627
    nimbus.childopts: "-Xmx1024m"
    nimbus.task.timeout.secs: 30

# In ConfigMap template
{{- if .Values.nimbus.enabled }}
# Nimbus configuration
{{- if .Values.nimbus.extraConfig }}
{{- range $key, $value := .Values.nimbus.extraConfig }}
{{ $key }}: {{ $value | toJson }}
{{- end }}
{{- end }}
{{- end }}
```