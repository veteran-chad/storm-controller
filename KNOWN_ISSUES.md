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