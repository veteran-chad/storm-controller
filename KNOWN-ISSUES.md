# Known Issues

This document tracks known issues discovered during testing and development of the Storm Kubernetes Controller.

## Controller Issues

### 1. Configuration Type Handling
**Issue**: Storm requires specific data types for configuration parameters, but Kubernetes ConfigMaps store everything as strings.

**Symptoms**: 
- Error: `Field TOPOLOGY_DEBUG must be of type class java.lang.Boolean. Object: true actual type: class java.lang.String`
- Topology submission fails with type validation errors

**Status**: ✅ Fixed

**Solution**: 
- Modified `buildSubmitCommand` in both controllers to properly handle data types:
  - Boolean values (`true`/`false`) are passed without quotes
  - String values are quoted (except for `topology.version` which requires special handling)
  - Numbers are passed without quotes

**Files Modified**:
- `src/controllers/stormtopology_controller.go`
- `src/controllers/stormtopology_controller_statemachine.go`

---

### 2. Worker Pod ConfigMap Reference Issue
**Issue**: Worker pods were referencing a hardcoded configmap name "storm-config" instead of using the actual configmap name from the StormCluster resource.

**Symptoms**:
- Worker pods stuck in `ContainerCreating` state
- Error: `MountVolume.SetUp failed for volume "storm-config" : configmap "storm-config" not found`

**Status**: ✅ Fixed

**Solution**: 
- Updated `buildWorkerPodSpec` method to check cluster management mode and use the correct configmap name from `cluster.Spec.ResourceNames.ConfigMap`

**Files Modified**:
- `src/controllers/stormworkerpool_controller_statemachine.go`
- `src/controllers/stormworkerpool_controller_enhanced.go`

---

### 3. Topology Version Not Appearing in Storm API
**Issue**: The `topology.version` configuration parameter was being passed during submission but wasn't appearing in the Storm API response.

**Symptoms**:
- `topologyVersion: null` in Storm API response
- Version not visible in Storm UI
- Configuration section doesn't include `topology.version`

**Status**: ✅ Fixed (as part of configuration type handling)

**Root Cause**: 
- Initially thought Storm wasn't persisting custom configuration parameters
- Actually was due to the string quoting issue - once properly quoted, Storm recognizes and stores the version

**Solution**: 
- Fixed as part of the configuration type handling solution above
- `topology.version` is now properly quoted as a string and appears in Storm API responses

---

## Storm Version Compatibility

### 1. Storm Version Inconsistency
**Issue**: Different Storm versions were being used across components (2.4.0 in Helm chart, 2.8.1 in Dockerfile).

**Symptoms**:
- Potential compatibility issues
- Confusion about which Storm version is actually running

**Status**: ✅ Fixed

**Solution**: 
- Updated all components to use Storm 2.8.1:
  - `charts/storm-kubernetes/values.yaml`: Changed image tag from 2.4.0 to 2.8.1
  - `charts/storm-kubernetes/Chart.yaml`: Updated appVersion from 2.6.0 to 2.8.1
  - Updated test topology JAR URLs to use 2.8.1 version

---

## State Machine Controller Issues

### 1. InternalState Field Warning
**Issue**: Controller logs showed warning about unknown field `status.internalState`.

**Symptoms**:
- Warning in logs: `KubeAPIWarningLogger unknown field "status.internalState"`
- State machine controller couldn't properly track state transitions

**Status**: ✅ Fixed

**Solution**: 
- The field existed in the CRD types but the CRD manifest wasn't regenerated
- Ran `make manifests` and reapplied the CRD

---

## Deployment Issues

### 1. Multiple Values Files Confusion
**Issue**: Multiple Helm values files (values-dev.yaml, values-controller-test.yaml, etc.) causing confusion about which to use.

**Status**: ✅ Fixed

**Solution**: 
- Consolidated to single `storm-local-values.yaml` for local testing
- Created `scripts/deploy-local.sh` for consistent deployment
- Removed redundant values files
- Updated documentation

---

## Pending Issues

### 1. Storm API Slot Reporting
**Issue**: Storm API reports 0 slots even when supervisors are running.

**Symptoms**:
- `totalSlots: 0` in cluster info
- Supervisor slots not being detected properly

**Status**: 🔄 Pending investigation

**Workaround**: 
- Topologies still deploy successfully despite 0 slots being reported
- This appears to be a reporting issue rather than functional issue

---

### 2. Resource Conflict Errors
**Issue**: Occasional "the object has been modified" errors during reconciliation.

**Symptoms**:
- Error: `Operation cannot be fulfilled on stormtopologies.storm.apache.org "wordcount-test": the object has been modified`
- Usually happens during rapid state transitions

**Status**: 🔄 Needs investigation

**Impact**: 
- Low - reconciliation retries automatically and eventually succeeds
- May cause slight delays in topology deployment

---

## Testing Notes

### Local Testing Setup
- Always use `storm-system` namespace
- Always run cleanup before deploying: `bash scripts/storm-controller-cleanup.sh storm-system`
- Use `storm-local-values.yaml` for consistent configuration
- Deploy with: `bash scripts/deploy-local.sh`

### Common Test Scenarios
1. **Basic topology deployment**: `kubectl apply -f examples/wordcount-topology.yaml`
2. **Version update**: Modify `topology.version` in the YAML and reapply
3. **Topology deletion**: `kubectl delete stormtopology -n storm-system wordcount-test`

---

## Debugging Tips

### Useful Commands
```bash
# Check controller logs
kubectl logs -n storm-system deployment/storm-cluster-storm-kubernetes-controller --tail=50

# Check Storm topology status
curl -s http://localhost:8080/api/v1/topology/summary | jq

# Check specific topology configuration
curl -s http://localhost:8080/api/v1/topology/<topology-id> | jq '.configuration'

# Port forward to Storm UI
kubectl port-forward --namespace storm-system svc/storm-cluster-storm-kubernetes-ui 8080:8080
```

### Common Log Patterns to Search
- `"Submitting topology"` - Find topology submission attempts
- `"State transition"` - Track state machine transitions
- `"ERROR"` - Find errors
- `"topology.version"` - Track version handling

---

*Last Updated: 2025-07-07*