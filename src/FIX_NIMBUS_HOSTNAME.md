# Nimbus Hostname Fix

## Issue
The Storm topology controller was constructing an incorrect Nimbus hostname when submitting topologies. It was using:
```
test-cluster-storm-kubernetes-nimbus.storm-system.svc.cluster.local
```

But the actual service name is:
```
test-cluster-nimbus.storm-system.svc.cluster.local
```

## Root Cause
In `controllers/stormtopology_controller.go`, line 793, the `buildSubmitCommand` method was incorrectly adding `-storm-kubernetes` to the Nimbus hostname:

```go
nimbusHost := fmt.Sprintf("%s-storm-kubernetes-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace)
```

## Fix
Changed line 793 to:
```go
nimbusHost := fmt.Sprintf("%s-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace)
```

## Testing
After applying this fix:
1. Rebuild the controller: `make build`
2. Build Docker image: `make docker-build IMG=storm-controller:latest`
3. Deploy the updated image to your cluster
4. Submit a test topology:
   ```yaml
   apiVersion: storm.apache.org/v1beta1
   kind: StormTopology
   metadata:
     name: wordcount-test
     namespace: storm-system
   spec:
     clusterRef: test-cluster
     topology:
       name: wordcount
       jar:
         url: "https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.8.1/storm-starter-2.8.1.jar"
       mainClass: "org.apache.storm.starter.WordCountTopology"
       config:
         topology.version: "2.0.0"
         topology.workers: "1"
         topology.debug: "false"
   ```

The topology should now successfully connect to Nimbus and submit.