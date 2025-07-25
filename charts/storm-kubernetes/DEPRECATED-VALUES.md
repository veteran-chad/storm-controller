# Deprecated Values in Storm Kubernetes Helm Chart

This document lists values that are deprecated or no longer in use after the configuration refactoring to use environment variables.

## Deprecated Values

### 1. `cluster.logFormat`
- **Status**: Deprecated, no longer used
- **Previous Purpose**: Set log format (text/json) for all Storm components
- **Replacement**: Use `extraEnvVars` per component to set `LOG_FORMAT`
- **Example**:
  ```yaml
  # Old way (deprecated)
  cluster:
    logFormat: "json"
  
  # New way
  nimbus:
    extraEnvVars:
      - name: LOG_FORMAT
        value: "json"
  supervisor:
    extraEnvVars:
      - name: LOG_FORMAT
        value: "json"
  ui:
    extraEnvVars:
      - name: LOG_FORMAT
        value: "json"
  ```

### 2. `clusterConfig` (entire section)
- **Status**: Deprecated, no longer used
- **Previous Purpose**: Provided a comprehensive Storm configuration section
- **Replacement**: Use `cluster.extraConfig`, `nimbus.extraConfig`, `supervisor.extraConfig`, and `ui.extraConfig`
- **Note**: All configuration is now handled through environment variables in the `configmap-env.yaml`
- **Example**:
  ```yaml
  # Old way (deprecated)
  clusterConfig:
    storm.log.level: "INFO"
    topology.workers: 2
    nimbus.childopts: "-Xmx1024m"
  
  # New way
  cluster:
    extraConfig:
      storm.log.level: "INFO"
      topology.workers: 2
  nimbus:
    extraConfig:
      nimbus.childopts: "-Xmx1024m"
  ```

## Removed Helper Functions

The following Helm template helper functions have been removed as they are no longer needed:

1. **`storm.renderClusterConfig`**
   - Previously used to render the deprecated `clusterConfig` section
   - No longer needed as configuration is handled via environment variables

2. **`storm.configToEnv`**
   - Previously used to convert configuration to environment variables inline
   - Replaced by centralized environment variable handling in `configmap-env.yaml`

3. **`storm.supervisor.memoryConfig`**
   - Previously used to generate memory configuration
   - Replaced by direct calculation in `configmap-env.yaml`

## Migration Timeline

- **Current Release**: Values are marked as deprecated but still present in values.yaml
- **Future Release**: These values will be completely removed from values.yaml
- **Action Required**: Update your values files to use the new configuration methods before upgrading to future releases

## Benefits of the New Approach

1. **Cleaner Configuration**: All Storm configuration is now managed through environment variables
2. **Better GitOps**: Configuration changes via ConfigMap don't require pod restarts (for new pods)
3. **Consistent Patterns**: All components follow the same configuration pattern
4. **Easier Updates**: Environment variables in ConfigMap can be updated independently
5. **Reduced Complexity**: Removed duplicate configuration paths and helper functions