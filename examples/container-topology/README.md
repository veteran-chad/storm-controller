# Container-Based Storm Topology Deployment

This example demonstrates how to deploy Storm topologies using container images instead of JAR URLs. This approach provides better version control, security, and CI/CD integration.

## Quick Start

### 1. Build Your Topology Container

```dockerfile
# Dockerfile
FROM scratch
COPY target/my-topology-1.0.jar /app/topology.jar
COPY lib/ /app/lib/
```

```bash
# Build and push the image
docker build -t myregistry.com/storm-topologies/wordcount:v1.0.0 .
docker push myregistry.com/storm-topologies/wordcount:v1.0.0
```

### 2. Deploy the Topology

```yaml
# topology.yaml
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount-container
spec:
  clusterRef: storm-cluster
  topology:
    name: wordcount
    jar:
      container:
        image: "myregistry.com/storm-topologies/wordcount:v1.0.0"
        path: "/app/topology.jar"
        extractionMode: "job"
    mainClass: com.example.WordCountTopology
```

```bash
kubectl apply -f topology.yaml
```

## Extraction Modes

### Job Mode (Recommended for Production)

Uses a Kubernetes Job to extract the JAR to persistent storage.

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    extractionMode: "job"
    extractionTimeoutSeconds: 300
```

**Benefits:**
- One-time extraction per JAR version
- Shared storage for multiple workers
- Better resource utilization

### Init Container Mode

Extracts JAR using init containers in each worker pod.

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    extractionMode: "initContainer"
```

**Benefits:**
- No external dependencies
- Fast worker startup (no network downloads)
- Works with any storage

### Sidecar Mode

Runs JAR container as a sidecar alongside Storm workers.

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    extractionMode: "sidecar"
```

**Benefits:**
- Hot reloading capability
- Good for development
- Real-time JAR updates

## Security Features

### Image Signing and Verification

```yaml
jar:
  container:
    image: "myregistry.com/topology@sha256:abc123..."
    checksum:
      algorithm: sha256
      value: "def456789..."
```

### Private Registry Authentication

```yaml
jar:
  container:
    image: "private-registry.com/topology:v1.0.0"
    pullSecrets:
    - name: registry-secret
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
```

## Advanced Configuration

### Resource Management

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 256Mi
```

### Environment Variables

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    env:
    - name: JAVA_OPTS
      value: "-Xmx128m"
    - name: TOPOLOGY_VERSION
      value: "v1.0.0"
```

### Volume Mounts

```yaml
jar:
  container:
    image: "myregistry.com/topology:v1.0.0"
    volumeMounts:
    - name: config
      mountPath: /etc/topology
    - name: cache
      mountPath: /tmp/cache
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Deploy Storm Topology

on:
  push:
    branches: [main]
    paths: ['src/topologies/**']

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Build JAR
      run: mvn clean package
    
    - name: Build Container
      run: |
        docker build -t $REGISTRY/topology:$GITHUB_SHA .
        docker push $REGISTRY/topology:$GITHUB_SHA
    
    - name: Deploy to Kubernetes
      run: |
        kubectl patch stormtopology my-topology \
          --type merge \
          -p '{"spec":{"topology":{"jar":{"container":{"image":"'$REGISTRY/topology:$GITHUB_SHA'"}}}}}'
```

### GitOps with ArgoCD

```yaml
# application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: storm-topologies
spec:
  project: default
  source:
    repoURL: https://github.com/company/storm-topologies
    targetRevision: HEAD
    path: k8s/
  destination:
    server: https://kubernetes.default.svc
    namespace: storm-production
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Best Practices

### 1. Image Optimization

```dockerfile
# Multi-stage build for smaller images
FROM maven:3.8-openjdk-11-slim AS builder
COPY pom.xml .
COPY src ./src
RUN mvn clean package -DskipTests

FROM scratch
COPY --from=builder target/topology.jar /app/topology.jar
```

### 2. Version Management

```bash
# Use semantic versioning
docker tag myregistry.com/topology:latest myregistry.com/topology:v1.2.3
docker tag myregistry.com/topology:latest myregistry.com/topology:v1.2
docker tag myregistry.com/topology:latest myregistry.com/topology:v1
```

### 3. Security Scanning

```yaml
# In CI pipeline
- name: Scan container image
  uses: anchore/scan-action@v3
  with:
    image: myregistry.com/topology:${{ github.sha }}
    fail-build: true
    severity-cutoff: high
```

### 4. Resource Optimization

```yaml
# Right-size extraction resources
jar:
  container:
    resources:
      requests:
        cpu: 50m        # Minimal for extraction
        memory: 64Mi
      limits:
        cpu: 200m       # Burst for large JARs
        memory: 128Mi
```

## Troubleshooting

### Check Extraction Job Status

```bash
kubectl get jobs -l storm.apache.org/topology=my-topology
kubectl logs job/my-topology-jar-extractor
```

### Verify JAR Extraction

```bash
kubectl exec -it storm-supervisor-0 -- ls -la /topology-jars/my-topology/
```

### Debug Container Issues

```bash
# Check image pull status
kubectl describe pod my-topology-jar-extractor-xxx

# Test image locally
docker run --rm myregistry.com/topology:v1.0.0 ls -la /app/
```

### Monitor Metrics

```bash
kubectl port-forward svc/storm-controller-metrics 8080:8080
curl http://localhost:8080/metrics | grep jar_extraction
```

## Examples

See the following files for complete examples:

- `topology.yaml` - Basic container topology
- `advanced-topology.yaml` - Production configuration
- `ci-pipeline.yaml` - Complete CI/CD pipeline
- `Dockerfile` - Container image build