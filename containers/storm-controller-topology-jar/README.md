# Storm Topology JAR Container

This directory contains the base Docker image for packaging Apache Storm topology JARs for deployment via the Storm Kubernetes controller.

## Overview

The Storm controller supports extracting topology JARs from container images as one of the JAR source methods. This base image provides a secure, minimal container for packaging your topology JARs.

## Features

- Minimal Alpine Linux base with Java 17 runtime
- Non-root user execution for security
- Built-in JAR validation and checksum calculation
- Flexible entrypoint script for various extraction modes
- Multi-stage build support for efficient images

## Building Your Topology Image

### Method 1: Using the Base Image

Create a `Dockerfile` in your topology project:

```dockerfile
FROM docker.io/veteranchad/storm-controller-topology-jar:latest

# Copy your topology JAR
COPY --chown=storm:storm target/my-topology-1.0.0.jar /storm/jars/topology.jar

# Optional: Set metadata
ENV TOPOLOGY_NAME="my-topology"
ENV TOPOLOGY_VERSION="1.0.0"
```

Build and push:

```bash
docker build -t myregistry/my-topology:1.0.0 .
docker push myregistry/my-topology:1.0.0
```

### Method 2: Multi-Stage Build

For smaller images, use multi-stage builds:

```dockerfile
# Build stage
FROM maven:3.8-openjdk-17 AS builder
WORKDIR /build
COPY pom.xml .
COPY src ./src
RUN mvn clean package

# Runtime stage
FROM docker.io/veteranchad/storm-controller-topology-jar:latest
COPY --from=builder --chown=storm:storm /build/target/my-topology-*.jar /storm/jars/topology.jar
```

## Using in Storm Controller

Reference your container image in the StormTopology resource:

```yaml
apiVersion: storm.apache.org/v1alpha1
kind: StormTopology
metadata:
  name: my-topology
spec:
  stormCluster: my-storm-cluster
  jarSource:
    container:
      image: myregistry/my-topology:1.0.0
      pullPolicy: IfNotPresent
      imagePullSecrets:
        - name: registry-credentials
      extractionMode: Job  # Options: Job, InitContainer, Sidecar
  checksum:
    type: sha256
    value: "abc123..."  # Optional: Verify JAR integrity
```

## Entrypoint Commands

The container supports several operational modes:

### Extract Mode
Extracts the JAR to a specified path:

```bash
docker run -e EXTRACTION_PATH=/output -v /host/path:/output \
  myregistry/my-topology:1.0.0 extract
```

### Validate Mode
Validates the JAR and optionally checks its checksum:

```bash
docker run -e EXPECTED_CHECKSUM=abc123 -e CHECKSUM_TYPE=sha256 \
  myregistry/my-topology:1.0.0 validate
```

### Info Mode
Displays information about the packaged JAR:

```bash
docker run myregistry/my-topology:1.0.0 info
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TOPOLOGY_JAR_PATH` | Path to the topology JAR inside container | `/storm/jars/topology.jar` |
| `EXTRACTION_PATH` | Directory to extract JAR to (extract mode) | - |
| `WRITE_CHECKSUM` | Write checksum file during extraction | - |
| `EXPECTED_CHECKSUM` | Expected checksum for validation | - |
| `CHECKSUM_TYPE` | Type of checksum (sha256, sha512, md5) | `sha256` |

## Building the Base Image

To build the base image locally:

```bash
cd containers/storm-controller-topology-jar
docker build -t storm-controller-topology-jar:latest .
```

## Security Considerations

1. **Non-root User**: The container runs as user `storm` (UID 1000)
2. **Minimal Attack Surface**: Alpine Linux with only essential packages
3. **Read-only Filesystem**: Compatible with read-only root filesystem
4. **No Shell by Default**: Uses `sleep infinity` to prevent execution

## Examples

### Private Registry with Authentication

```yaml
apiVersion: storm.apache.org/v1alpha1
kind: StormTopology
metadata:
  name: secure-topology
spec:
  stormCluster: production-cluster
  jarSource:
    container:
      image: private.registry.io/storm/my-topology:1.0.0
      imagePullSecrets:
        - name: private-registry-auth
      extractionMode: Job
  checksum:
    type: sha256
    value: "d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2"
```

### Using with CI/CD

GitLab CI example:

```yaml
build-topology:
  stage: build
  script:
    - mvn clean package
    - docker build -t $CI_REGISTRY_IMAGE/topology:$CI_COMMIT_SHA .
    - docker push $CI_REGISTRY_IMAGE/topology:$CI_COMMIT_SHA
    - SHA256=$(docker run --rm $CI_REGISTRY_IMAGE/topology:$CI_COMMIT_SHA info | grep SHA256 | awk '{print $2}')
    - echo "TOPOLOGY_CHECKSUM=$SHA256" >> deploy.env
  artifacts:
    reports:
      dotenv: deploy.env
```

## Troubleshooting

### JAR Not Found
- Ensure the JAR is copied to `/storm/jars/topology.jar`
- Check file permissions (should be owned by `storm:storm`)

### Extraction Fails
- Verify the extraction path is mounted and writable
- Check container logs for detailed error messages

### Checksum Mismatch
- Regenerate checksum from the source JAR
- Ensure the same checksum algorithm is used

## Best Practices

1. **Version Tags**: Always use specific version tags, not `latest`
2. **Checksum Verification**: Enable checksum validation in production
3. **Small Images**: Use multi-stage builds to minimize image size
4. **Security Scanning**: Scan images for vulnerabilities before deployment
5. **Metadata Labels**: Add descriptive labels for better management