# Container-Based Storm Topology Deployments

This guide explains how to deploy Storm topologies using container images instead of URL-based JARs.

## Why Use Container-Based Deployments?

1. **Version Control**: Topology JARs are versioned alongside container images
2. **Security**: No need to expose JAR files via HTTP/HTTPS
3. **CI/CD Integration**: Natural fit with container-based CI/CD pipelines
4. **Dependency Management**: All dependencies packaged together
5. **Air-gapped Environments**: Works in environments without internet access

## How It Works

The Storm controller can extract JAR files from container images using two methods:

### 1. Copy Mode (Default)
- Creates a Kubernetes Job to extract the JAR
- Copies the JAR to a shared volume
- Suitable for most use cases

### 2. Init Container Mode
- Uses an init container in the topology pod
- Extracts JAR directly before topology starts
- Lower overhead for frequently updated topologies

## Examples

### Using Storm Base Image

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount-container
  namespace: storm-system
spec:
  clusterRef: test-cluster
  topology:
    name: wordcount-container
    jar:
      container:
        image: apache/storm:2.8.1
        path: /apache-storm/examples/storm-starter/storm-starter-topologies-2.8.1.jar
        extractionMode: copy
    mainClass: "org.apache.storm.starter.WordCountTopology"
    config:
      topology.workers: "2"
```

### Using Custom Topology Image

```yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: my-topology
  namespace: storm-system
spec:
  clusterRef: test-cluster
  topology:
    name: my-topology
    jar:
      container:
        image: mycompany/my-storm-topology:v1.2.3
        path: /app/topology.jar
        extractionMode: init-container
        imagePullSecrets:
          - name: registry-credentials
    mainClass: "com.mycompany.MyTopology"
```

## Building a Topology Image

### Simple Dockerfile

```dockerfile
FROM openjdk:11-jre-slim
COPY target/my-topology.jar /app/topology.jar
```

### Multi-stage Build

```dockerfile
# Build stage
FROM maven:3.8-openjdk-11 AS builder
WORKDIR /build
COPY pom.xml .
COPY src ./src
RUN mvn clean package -DskipTests

# Runtime stage
FROM busybox:latest
COPY --from=builder /build/target/my-topology-*.jar /app/topology.jar
```

## Best Practices

1. **Use Specific Tags**: Always use specific image tags, not `latest`
2. **Minimal Images**: Use minimal base images (busybox, distroless) for the final stage
3. **Label Your Images**: Add metadata labels for better tracking
4. **Security Scanning**: Scan images for vulnerabilities
5. **Size Optimization**: Keep images small for faster extraction

## Troubleshooting

### JAR Extraction Failed
- Check the image exists and is accessible
- Verify the path inside the container is correct
- Check pod permissions and security contexts

### Image Pull Errors
- Verify imagePullSecrets are configured correctly
- Check registry connectivity
- Ensure the service account has pull permissions

### Extraction Job Stuck
- Check resource quotas
- Verify the extraction PVC has enough space
- Look at job logs: `kubectl logs -n storm-system job/topology-jar-extract-<name>`

## Advanced Configuration

### Custom Extraction Command

```yaml
jar:
  container:
    image: mycompany/topology:v1.0.0
    path: /app/topology.jar
    extractionMode: copy
    command: ["/bin/sh", "-c"]
    args: ["cp /app/*.jar /extracted/"]
```

### Resource Limits for Extraction

```yaml
jar:
  container:
    image: mycompany/topology:v1.0.0
    path: /app/topology.jar
    resources:
      limits:
        memory: "256Mi"
        cpu: "500m"
      requests:
        memory: "128Mi"
        cpu: "100m"
```