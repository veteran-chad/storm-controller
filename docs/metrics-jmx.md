# JMX Metrics Configuration for Storm Controller

## Overview

This document describes how to configure JMX (Java Management Extensions) metrics collection for Apache Storm clusters managed by the Storm Controller. JMX provides detailed insights into JVM performance and Storm-specific metrics at the worker level.

## Current State

The default Storm container images (`storm:2.8.1`) include basic JMX support through the `metrics-jmx-4.2.30.jar` library, but do not include a JMX exporter agent for external monitoring systems like Prometheus.

### Available JMX Components

- **Storm JMX Library**: `/apache-storm/lib/metrics-jmx-4.2.30.jar`
- **JDK JMX Configuration**: `/opt/java/openjdk/conf/management/jmxremote.*`
- **No JMX Exporter**: No Prometheus JMX exporter or similar agent is pre-installed

## Configuring JMX for Storm Workers

### Worker Child Options

To enable JMX monitoring for Storm workers, you can add JVM arguments to `supervisor.worker.childopts`:

```yaml
supervisor.worker.childopts: "-Xmx1024m -javaagent:/jmx/agent.jar=2%ID%:/jmx/config.yaml"
```

#### Breaking Down the Configuration

1. **`-javaagent:/jmx/agent.jar`**
   - Loads a Java agent at JVM startup
   - Typically a JMX exporter (e.g., Prometheus JMX Exporter)
   - Must be mounted into the container at this path

2. **`=2%ID%:`**
   - Port configuration pattern where `%ID%` is replaced with the worker port offset
   - Examples:
     - Worker on port 6700 → JMX port 20000
     - Worker on port 6701 → JMX port 20001
     - Worker on port 6702 → JMX port 20002

3. **`:/jmx/config.yaml`**
   - Path to the JMX exporter configuration file
   - Defines which metrics to expose and how to format them

## Prerequisites for JMX Agent Setup

### 1. Mount JMX Agent JAR

Create a ConfigMap or use an init container to provide the JMX agent:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jmx-agent-config
  namespace: storm-system
data:
  config.yaml: |
    ---
    startDelaySeconds: 0
    ssl: false
    lowercaseOutputName: true
    lowercaseOutputLabelNames: true
    whitelistObjectNames:
      - "java.lang:type=Memory"
      - "java.lang:type=GarbageCollector,*"
      - "java.lang:type=Threading"
      - "java.lang:type=Runtime"
      - "java.lang:type=OperatingSystem"
      - "metrics:name=*"
    rules:
      - pattern: 'java.lang<type=Memory><HeapMemoryUsage>(\w+)'
        name: jvm_memory_heap_$1
        type: GAUGE
      - pattern: 'java.lang<type=GarbageCollector, name=(.+)><>CollectionCount'
        name: jvm_gc_collection_count
        type: COUNTER
        labels:
          gc: "$1"
```

### 2. Modify Storm Cluster Deployment

Add volume mounts to the supervisor deployment:

```yaml
volumes:
  - name: jmx-agent
    emptyDir: {}
  - name: jmx-config
    configMap:
      name: jmx-agent-config
      
initContainers:
  - name: jmx-agent-download
    image: busybox
    command: 
      - sh
      - -c
      - |
        wget -O /jmx/agent.jar https://repo1.maven.org/maven2/io/prometheus/jmx/jmx_prometheus_javaagent/0.19.0/jmx_prometheus_javaagent-0.19.0.jar
    volumeMounts:
      - name: jmx-agent
        mountPath: /jmx
        
containers:
  - name: supervisor
    volumeMounts:
      - name: jmx-agent
        mountPath: /jmx
        readOnly: true
      - name: jmx-config
        mountPath: /jmx/config.yaml
        subPath: config.yaml
```

### 3. Expose JMX Ports

Update the supervisor pod specification to expose JMX ports:

```yaml
ports:
  - containerPort: 6700  # Worker port
  - containerPort: 6701
  - containerPort: 6702
  - containerPort: 6703
  - containerPort: 20000 # JMX port for worker 6700
  - containerPort: 20001 # JMX port for worker 6701
  - containerPort: 20002 # JMX port for worker 6702
  - containerPort: 20003 # JMX port for worker 6703
```

### 4. Create Service for Metrics Collection

```yaml
apiVersion: v1
kind: Service
metadata:
  name: storm-workers-metrics
  namespace: storm-system
  labels:
    app: storm
    component: worker-metrics
spec:
  selector:
    app: storm
    component: supervisor
  ports:
    - name: jmx-6700
      port: 20000
      targetPort: 20000
    - name: jmx-6701
      port: 20001
      targetPort: 20001
    - name: jmx-6702
      port: 20002
      targetPort: 20002
    - name: jmx-6703
      port: 20003
      targetPort: 20003
```

## Metrics Available via JMX

### JVM Metrics
- **Memory Usage**: Heap and non-heap memory statistics
- **Garbage Collection**: GC counts, duration, and memory reclaimed
- **Thread Statistics**: Thread counts, states, and CPU usage
- **Class Loading**: Loaded classes count
- **CPU Usage**: Process and system CPU utilization

### Storm Worker Metrics
- **Tuple Processing**:
  - Emitted tuple count
  - Transferred tuple count
  - Acked/failed tuple count
  - Process latency
- **Queue Metrics**:
  - Receive queue size
  - Send queue size
  - Overflow count
- **Executor Statistics**:
  - Execute count
  - Execute latency
  - Capacity utilization
- **Error Tracking**:
  - Error count by component
  - Last error details

## Integration with Prometheus

To scrape these metrics with Prometheus, add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'storm-workers'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - storm-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: storm
      - source_labels: [__meta_kubernetes_pod_label_component]
        action: keep
        regex: supervisor
      - source_labels: [__meta_kubernetes_pod_container_port_number]
        action: keep
        regex: "2000[0-3]"
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
        target_label: __address__
        regex: (.+);(.+);(.+)
        replacement: ${2}.${1}.pod.cluster.local:${3}
```

## Alternative: Using Storm's Built-in Metrics Reporters

Storm also supports built-in metrics reporters that can be configured without JMX:

```yaml
storm.daemon.metrics.reporter.plugins:
  - "org.apache.storm.daemon.metrics.reporters.JmxPreparableReporter"
storm.daemon.metrics.reporter.interval.secs: 10
```

However, this requires custom integration to expose metrics in a format consumable by modern monitoring systems.

## Security Considerations

1. **Authentication**: Configure JMX authentication if exposing ports outside the cluster
2. **Network Policies**: Restrict access to JMX ports using Kubernetes NetworkPolicies
3. **TLS/SSL**: Enable SSL for JMX connections in production environments

## Troubleshooting

### Verify JMX Agent is Loaded

Check worker process arguments:
```bash
kubectl exec -n storm-system <supervisor-pod> -- ps aux | grep javaagent
```

### Test JMX Port Connectivity

```bash
kubectl port-forward -n storm-system <supervisor-pod> 20000:20000
curl http://localhost:20000/metrics
```

### Common Issues

1. **Agent JAR not found**: Ensure the JAR is properly mounted
2. **Port conflicts**: Verify port mappings don't overlap
3. **Configuration errors**: Check agent logs for parsing errors
4. **Memory overhead**: JMX agent adds ~50MB memory overhead per worker

## Future Enhancements

1. **Pre-built Images**: Create Storm images with JMX exporters pre-installed
2. **Operator Integration**: Add JMX configuration options to StormCluster CRD
3. **Auto-discovery**: Implement automatic port assignment for workers
4. **Grafana Dashboards**: Provide pre-built dashboards for Storm worker metrics