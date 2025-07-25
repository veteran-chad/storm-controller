# Migration Guide: Storm Kubernetes Helm Chart

## Migrating from Previous Version to Current Version

This guide helps you migrate your values files from the previous version of the Storm Kubernetes helm chart to the current version that uses environment variable-based configuration.

### Overview of Changes

1. **Configuration Method**: Storm configuration is now managed through environment variables stored in a ConfigMap
2. **Logging**: The `cluster.logFormat` setting is deprecated; use `LOG_FORMAT` environment variable instead
3. **Custom storm.yaml**: Now optional and only created when `cluster.stormYaml` is provided
4. **Container Image**: Updated to use new Storm container with tag `2.8.1-17-jre`

### Migration Steps

#### 1. Update Image Tags

Change your Storm image tags from `2.8.1` to `2.8.1-17-jre`:

**Before:**
```yaml
nimbus:
  image:
    tag: 2.8.1

supervisor:
  image:
    tag: 2.8.1

ui:
  image:
    tag: 2.8.1
```

**After:**
```yaml
nimbus:
  image:
    tag: 2.8.1-17-jre

supervisor:
  image:
    tag: 2.8.1-17-jre

ui:
  image:
    tag: 2.8.1-17-jre
```

#### 2. Update Logging Configuration

The `cluster.logFormat` is now deprecated. Use environment variables instead:

**Before:**
```yaml
cluster:
  logFormat: "json"
```

**After:**
```yaml
# Option 1: Set globally for all components
cluster:
  extraConfig:
    # This will set LOG_FORMAT env var for all components
    log.format: "json"  # Note: this is a pseudo-config, not a real Storm config

# Option 2: Set per component (recommended)
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

#### 3. Review Your Configuration Structure

Your existing `extraConfig` sections will continue to work and are automatically converted to environment variables:

**Existing (still works):**
```yaml
cluster:
  extraConfig:
    storm.log.level: "INFO"
    topology.workers: 2

nimbus:
  extraConfig:
    nimbus.childopts: "-Xmx1024m"
    nimbus.task.timeout.secs: 30

supervisor:
  extraConfig:
    supervisor.childopts: "-Xmx256m"
    supervisor.worker.timeout.secs: 30

ui:
  extraConfig:
    ui.childopts: "-Xmx768m"
```

These are automatically converted to environment variables:
- `storm.log.level` → `STORM_STORM__LOG__LEVEL`
- `nimbus.childopts` → `STORM_NIMBUS__CHILDOPTS`
- etc.

#### 4. Custom storm.yaml (Optional)

If you were using a custom ConfigMap for storm.yaml, you can now use the built-in support:

**Before (using custom ConfigMap):**
```yaml
# You had to create your own ConfigMap and mount it
extraVolumes:
  - name: custom-config
    configMap:
      name: my-storm-config

extraVolumeMounts:
  - name: custom-config
    mountPath: /conf
```

**After (using built-in support):**
```yaml
cluster:
  stormYaml: |
    storm.zookeeper.servers:
      - "zookeeper"
    nimbus.seeds:
      - "nimbus"
    storm.log.dir: "/logs"
    storm.local.dir: "/data"
    # Your complete storm.yaml content here
```

### Example: Complete Migration

Here's a complete example of migrating a typical values file:

**Before (old version):**
```yaml
cluster:
  enabled: true
  logFormat: "json"
  extraConfig:
    storm.log.level: "INFO"

ui:
  enabled: true
  replicaCount: 1
  image:
    tag: 2.8.1
  extraConfig:
    ui.childopts: "-Xmx768m"

nimbus:
  enabled: true
  replicaCount: 1
  image:
    tag: 2.8.1
  extraConfig:
    nimbus.childopts: "-Xmx1024m"
    nimbus.task.timeout.secs: 30

supervisor:
  enabled: true
  replicaCount: 3
  image:
    tag: 2.8.1
  slotsPerSupervisor: 4
  memoryConfig:
    mode: "auto"
    memoryPerWorker: "1Gi"
  extraConfig:
    supervisor.childopts: "-Xmx256m"
```

**After (new version):**
```yaml
cluster:
  enabled: true
  # logFormat is deprecated, remove it
  extraConfig:
    storm.log.level: "INFO"

ui:
  enabled: true
  replicaCount: 1
  image:
    tag: 2.8.1-17-jre  # Updated tag
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"     # Moved from cluster.logFormat
  extraConfig:
    ui.childopts: "-Xmx768m"

nimbus:
  enabled: true
  replicaCount: 1
  image:
    tag: 2.8.1-17-jre  # Updated tag
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"     # Moved from cluster.logFormat
  extraConfig:
    nimbus.childopts: "-Xmx1024m"
    nimbus.task.timeout.secs: 30

supervisor:
  enabled: true
  replicaCount: 3
  image:
    tag: 2.8.1-17-jre  # Updated tag
  slotsPerSupervisor: 4
  memoryConfig:
    mode: "auto"
    memoryPerWorker: "1Gi"
  extraEnvVars:
    - name: LOG_FORMAT
      value: "json"     # Moved from cluster.logFormat
  extraConfig:
    supervisor.childopts: "-Xmx256m"
```

### Validation

After migration, validate your configuration:

1. **Dry run with helm template:**
   ```bash
   helm template my-storm . -f my-values.yaml --namespace storm-system
   ```

2. **Check the generated ConfigMaps:**
   - Look for `<release-name>-env` ConfigMap with all environment variables
   - If you provided `cluster.stormYaml`, check for the storm.yaml ConfigMap

3. **Verify environment variables:**
   ```bash
   # After deployment, check a pod's environment
   kubectl exec -n storm-system <nimbus-pod> -- env | grep STORM_
   ```

### Rollback Plan

If you need to rollback:

1. Keep your old values file as backup
2. The old configuration method (without the new features) still works
3. You can continue using the old image tags if needed

### Benefits of Migration

1. **Easier configuration updates**: Update the ConfigMap without redeploying pods
2. **Better GitOps**: Environment variables in ConfigMap are easier to track and audit
3. **New features**: Access to new Storm container features like flexible logging
4. **Cleaner configuration**: Less duplication across components

### Need Help?

If you encounter issues during migration:

1. Check the rendered templates: `helm template --debug`
2. Review the ConfigMap contents: `kubectl get cm <release>-env -o yaml`
3. Check pod logs for configuration errors: `kubectl logs <pod-name>`