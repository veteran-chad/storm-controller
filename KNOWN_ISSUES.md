# Known Issues

## Configuration Type Limitations

### ConfigMap Merge Behavior

The current configuration merging system has the following limitations:

1. **Type Detection**: The system uses string-based type inference to determine if a value should be treated as a boolean, number, or string. This can lead to edge cases where:
   - Numeric strings (e.g., "123abc") might be incorrectly classified
   - Boolean-like strings (e.g., "True", "FALSE") may not be recognized
   - Complex types (arrays, objects) require manual JSON formatting

2. **Type Preservation**: When merging configurations from multiple sources (operator defaults, CRD defaults, user config), type information can be lost since Kubernetes ConfigMaps store all values as strings.

3. **Storm CLI Compatibility**: The controller must format configuration values correctly for the Storm CLI:
   - Booleans must be passed without quotes: `true` not `"true"`
   - Numbers must be passed without quotes: `123` not `"123"`
   - Strings must be properly quoted: `"my-value"`

### Workarounds

1. **Explicit Type Hints**: For ambiguous values, use explicit formatting in your configuration:
   ```yaml
   stormConfig:
     # Clearly a string
     my.string.value: '"12345"'
     # Clearly a number
     my.number.value: '12345'
     # Clearly a boolean
     my.bool.value: 'true'
   ```

2. **Validation**: Always check the generated ConfigMap to ensure values are formatted correctly:
   ```bash
   kubectl get configmap <cluster-name>-config -o yaml
   ```

3. **Testing**: Test your configuration with a simple topology before deploying production workloads.

### Future Improvements

- Implement a schema-based configuration system with explicit type definitions
- Add configuration validation webhook
- Support for typed configuration in CRDs

## Test Status

### Successfully Fixed Tests
1. **"Should load operator config and apply defaults"** (`controllers/stormcluster_controller_test.go`)
   - Updated to properly create ConfigMap and test config loading
   - Now passing successfully

2. **"Should handle missing operator config gracefully"** (`controllers/stormcluster_controller_test.go`)
   - Updated to test fallback to default configuration
   - Fixed expected default values
   - Now passing successfully

### Tests with Fake Client Issues
The following tests fail due to limitations with the controller-runtime fake client when dealing with finalizers:

3. **"should handle full lifecycle from creation to running"** (`controllers/stormtopology_controller_test.go`)
   - Test logic is correct but fake client returns 404 after updating object with finalizer
   - This is a known limitation of the fake client

4. **"should handle full lifecycle from creation to ready"** (`controllers/stormworkerpool_controller_test.go`)
   - Test logic is correct but fake client returns 404 after updating object with finalizer
   - This is a known limitation of the fake client

## Root Cause
These tests were written for the previous architecture and are failing after the refactoring that:
- Separated operator deployment from cluster deployment
- Introduced operator-level configuration management
- Changed how defaults are applied to clusters

The tests appear to have issues with the fake client not properly handling object updates after adding finalizers, causing subsequent reads to fail.

## Resolution
These tests should be rewritten to:
1. Properly mock the configuration loading
2. Handle the new finalizer behavior with fake clients
3. Test the new configuration merging logic
4. Validate the new default application behavior

## Other Known Issues

### Storm API Slot Reporting

The Storm API sometimes reports incorrect slot information:
- Total slots may show as 0 even when supervisors are running
- Used/free slot counts may be inaccurate
- This appears to be a Storm API issue, not a controller issue

**Workaround**: Check the Storm UI for accurate slot information.

### Resource Conflict Errors

Occasional "Operation cannot be fulfilled" errors during reconciliation:
- These are typically transient and resolve on retry
- Caused by concurrent updates to the same resource
- The controller's retry mechanism handles these automatically

**Impact**: Minimal - may cause slight delays in reconciliation.

### ConfigMap Name References

When using `managementMode: reference`:
- The controller expects specific resource names
- ConfigMap must match the pattern or be explicitly specified
- Missing ConfigMap causes pod creation failures

**Solution**: Always verify ConfigMap names match expectations or use `resourceNames` to specify custom names.

### Topology Version Tracking

Topology versions must be specified in the configuration:
- The `spec.version` field tracks deployment versions
- The `topology.version` config tracks Storm's internal version
- Both should be kept in sync for clarity

**Best Practice**: Always update both version fields when deploying new topology versions.

## CI/CD Issues

### GitHub Actions Docker Build Cache

**Issue**: Docker builds in GitHub Actions may fail with 502 Bad Gateway errors when using the GitHub Actions cache (`type=gha`).

**Error Message**:
```
ERROR: failed to parse error response 502: <!DOCTYPE html>
```

**Root Cause**: GitHub's cache service experiences intermittent failures, returning HTML error pages instead of proper cache responses.

**Current Workaround**: The GitHub Actions cache has been disabled in all workflows. Docker builds will run without caching, which increases build times but ensures reliability.

**Future Fix**: Re-enable caching once GitHub resolves the infrastructure issues. The cache configuration can be restored by uncommenting the following lines in the workflow files:
```yaml
cache-from: type=gha,scope=buildkit
cache-to: type=gha,mode=max,scope=buildkit
```

**Impact**: Increased build times in CI/CD pipelines. No impact on functionality.

### Helm Chart Dependency Version Conflicts in CI

**Issue**: The CI workflow fails during the "Package Helm Charts" stage with Chart.lock out of sync errors when dynamically updating chart versions.

**Error Messages**:
```
Error: the lock file (Chart.lock) is out of sync with the dependencies file (Chart.yaml). Please update the dependencies
```
Or:
```
Error: could not download oci://registry-1.docker.io/veteranchad/storm-shared: failed to perform "FetchReference" on source: registry-1.docker.io/veteranchad/storm-shared:0.0.0-70f2bd3: not found
```

**Root Cause**: The CI workflow updates chart versions dynamically (e.g., to `0.0.0-<commit-hash>` for non-release builds), but:
1. This makes Chart.lock files out of sync with the modified Chart.yaml files
2. When updating storm-shared dependency versions to match, the dependency cannot be found in the registry because it hasn't been pushed yet
3. Various attempts to fix this (removing Chart.lock, using helm dependency update vs build, conditional logic) have not resolved the issue

**Current Workaround**: The CI workflow has been temporarily disabled and can only be triggered manually via workflow_dispatch. The release workflow is still active for tagged releases.

**Proposed Solutions**:
1. Use a local file repository during CI builds for storm-shared
2. Push storm-shared to a temporary tag first, then update dependencies
3. Don't update dependency versions for non-release builds
4. Use a different versioning strategy that doesn't require dynamic updates

**Impact**: PR builds don't automatically build and test changes. Manual testing required before merging.