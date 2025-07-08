# Storm Controller Testing Guide

This guide explains how to test various topology deployment methods once the controller fix is deployed.

## Prerequisites

1. Storm cluster running in Kubernetes
2. Storm operator with the Nimbus hostname fix deployed
3. Access to kubectl

## Test 1: URL-Based Topology

This tests the basic topology submission from a URL.

```bash
kubectl apply -f examples/wordcount-topology.yaml
```

Expected behavior:
- Topology should transition through states: Unknown → Validating → Downloading → Submitting → Running
- JAR should be downloaded to controller's cache
- Topology should appear in Storm UI

Verify:
```bash
# Check topology status
kubectl get stormtopology wordcount -n storm-system

# Check Storm UI
kubectl port-forward -n storm-system svc/test-cluster-ui 8080:8080
# Browse to http://localhost:8080

# Check logs
kubectl logs -n storm-system deployment/storm-operator-operator | grep wordcount
```

## Test 2: Container-Based Topology (Job Mode)

This tests JAR extraction from a container using a Kubernetes Job.

```bash
kubectl apply -f examples/wordcount-container-topology.yaml
```

Expected behavior:
- Extraction job should be created
- JAR should be extracted from container
- Topology should be submitted after extraction

Verify:
```bash
# Check extraction job
kubectl get jobs -n storm-system -l storm.apache.org/topology=wordcount-container

# Check job logs
kubectl logs -n storm-system job/jar-extract-wordcount-container

# Check topology status
kubectl get stormtopology wordcount-container -n storm-system -o yaml
```

## Test 3: Container-Based Topology (InitContainer Mode)

This tests JAR extraction using an init container.

First, create a custom topology with initContainer mode:

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount-init
  namespace: storm-system
spec:
  clusterRef: test-cluster
  topology:
    name: wordcount-init
    jar:
      container:
        image: apache/storm:2.8.1
        path: /apache-storm/examples/storm-starter/storm-starter-topologies-2.8.1.jar
        extractionMode: initContainer
    mainClass: "org.apache.storm.starter.WordCountTopology"
    config:
      topology.workers: "1"
```

Expected behavior:
- No extraction job created
- Init container extracts JAR before topology submission
- Faster startup for subsequent deployments

## Test 4: Topology Update

Test updating a running topology:

```bash
# First deploy version 1.0.0
kubectl apply -f examples/wordcount-topology.yaml

# Wait for it to be running
kubectl wait --for=condition=Ready stormtopology/wordcount -n storm-system

# Update the version
kubectl patch stormtopology wordcount -n storm-system --type merge -p '
{
  "spec": {
    "topology": {
      "config": {
        "topology.version": "2.0.0",
        "topology.workers": "3"
      }
    }
  }
}'
```

Expected behavior:
- Topology should transition to Updating state
- Old topology killed
- New topology submitted with updated config

## Test 5: Topology Deletion

Test proper cleanup:

```bash
kubectl delete stormtopology wordcount -n storm-system
```

Expected behavior:
- Finalizer should prevent immediate deletion
- Topology killed in Storm
- Resources cleaned up
- CR deleted after topology removed from Storm

## Test 6: Error Scenarios

### Invalid JAR URL
```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: invalid-url
  namespace: storm-system
spec:
  clusterRef: test-cluster
  topology:
    name: invalid-url
    jar:
      url: "https://invalid.example.com/nonexistent.jar"
    mainClass: "com.example.Invalid"
```

Expected: Topology should enter Failed state with download error

### Invalid Container Image
```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: invalid-image
  namespace: storm-system
spec:
  clusterRef: test-cluster
  topology:
    name: invalid-image
    jar:
      container:
        image: nonexistent/image:v1.0.0
        path: /app/topology.jar
    mainClass: "com.example.Invalid"
```

Expected: Extraction job should fail with image pull error

## Monitoring and Debugging

### Check Controller Logs
```bash
kubectl logs -n storm-system deployment/storm-operator-operator -f | grep -E "wordcount|ERROR"
```

### Check Topology Events
```bash
kubectl describe stormtopology wordcount -n storm-system
```

### Check Storm Cluster Status
```bash
# Storm cluster health
kubectl get stormcluster -n storm-system

# Nimbus logs
kubectl logs -n storm-system statefulset/test-cluster-nimbus

# Supervisor logs  
kubectl logs -n storm-system deployment/test-cluster-supervisor
```

### Check Metrics
```bash
# Port-forward to metrics endpoint
kubectl port-forward -n storm-system deployment/storm-operator-operator 8080:8080

# Check metrics
curl http://localhost:8080/metrics | grep storm_topology
```

## Common Issues

1. **Topology stuck in Pending**
   - Check if Storm cluster is ready
   - Verify ClusterRef is correct
   - Check controller logs for errors

2. **JAR extraction fails**
   - Verify image exists and is accessible
   - Check JAR path inside container
   - Review extraction job logs

3. **Topology submission fails**
   - Verify Nimbus is reachable
   - Check Storm logs for errors
   - Ensure JAR and main class are correct

4. **Topology not appearing in Storm UI**
   - Check if topology was successfully submitted
   - Verify Storm UI is accessible
   - Check Nimbus logs