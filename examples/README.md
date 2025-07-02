# Storm Controller Examples

This directory contains example configurations and deployments for the Storm Kubernetes controller.

## Directory Structure

```
examples/
├── clusters/          # StormCluster resource examples
├── topologies/        # StormTopology resource examples
├── container-topology/   # Example containerized topology with CI/CD
├── starter-topology/     # Basic Java topology example
└── storm-topology-example/  # Dockerfile example for topology packaging
```

## Quick Start Examples

### 1. Deploy a Storm Cluster Reference

```bash
# Create a reference to an existing Storm cluster
kubectl apply -f clusters/basic-cluster.yaml
```

### 2. Submit a Topology from URL

```bash
# Submit a topology using a JAR from URL
kubectl apply -f topologies/test-topology.yaml
```

### 3. Submit a Containerized Topology

```bash
# Build and deploy a containerized topology
cd container-topology
docker build -t my-topology:latest .
docker push my-registry/my-topology:latest

# Update the image in the YAML and apply
kubectl apply -f ../topologies/simple-container-topology.yaml
```

## Example Categories

### Cluster Examples (`clusters/`)

- `basic-cluster.yaml` - Minimal StormCluster configuration
- `simple-cluster.yaml` - Simple cluster with basic settings
- `storm-cluster-cr.yaml` - Full-featured cluster reference
- `stormcluster.yaml` - Production-ready cluster configuration
- `stormcluster-simple.yaml` - Simplified cluster for testing

### Topology Examples (`topologies/`)

- `test-topology.yaml` - Basic topology from URL
- `simple-container-topology.yaml` - Topology from container image
- `container-test-topology.yaml` - Container topology with checksum validation
- `test-topology-submission.yaml` - Advanced submission options
- `test-topology-with-version.yaml` - Versioned topology deployment
- `test-version-update.yaml` - Example of topology version update
- `test-helm-topology.yaml` - Topology for Helm deployments

### Container Examples

#### `container-topology/`
Complete example of building a containerized Storm topology with:
- Multi-stage Dockerfile
- GitLab CI/CD pipeline
- Checksum generation
- Private registry support

#### `storm-topology-example/`
Simple Dockerfile example for packaging existing JARs into containers.

#### `starter-topology/`
Basic Java Storm topology source code with Maven build configuration.

## Best Practices

1. **Always specify resource limits** in your topology configurations
2. **Use checksums** for container-based deployments in production
3. **Version your topologies** to enable smooth updates
4. **Test locally** before deploying to production clusters

## Deployment Workflow

1. **Prepare your topology JAR** (build from source or download)
2. **Choose deployment method**:
   - URL: Host JAR on accessible HTTP(S) server
   - Container: Package JAR in Docker image
   - ConfigMap/Secret: For small JARs (<1MB)
3. **Create StormTopology resource** using appropriate example as template
4. **Apply configuration**: `kubectl apply -f your-topology.yaml`
5. **Monitor deployment**: `kubectl get stormtopology -w`

## Troubleshooting

If your topology fails to deploy:

1. Check controller logs: `kubectl logs -n storm-system deployment/storm-controller`
2. Check topology status: `kubectl describe stormtopology your-topology`
3. Verify cluster health: `kubectl get stormcluster`
4. Ensure JAR is accessible from the cluster

## Additional Resources

- [Controller Architecture](../docs/controller-architecture.md)
- [Deployment Guide](../docs/DEPLOYMENT_SUMMARY.md)
- [Helm Chart Documentation](../docs/HELM_CHART_README.md)