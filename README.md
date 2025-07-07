# Storm Kubernetes Controller

A Kubernetes controller for managing Apache Storm deployments using Custom Resource Definitions (CRDs).

## Features

- **Declarative Topology Management**: Deploy and manage Storm topologies using Kubernetes resources
- **Version Management**: Automatic topology updates when version changes
- **Container-based JAR Deployment**: Support for JAR files packaged in container images
- **Reliable Deletion**: CLI-based topology deletion with proper cleanup
- **Comprehensive Logging**: Detailed logging of all operations

## Repository Structure

```
.
├── src/                    # Storm controller source code
│   ├── api/               # CRD API definitions
│   ├── controllers/       # Controller implementation
│   ├── pkg/              # Shared packages
│   └── Dockerfile        # Controller container image
├── charts/                # Helm charts
│   └── storm-kubernetes/ # Storm cluster Helm chart with CRDs
├── scripts/              # Utility scripts
│   ├── storm-controller-cleanup.sh
│   └── storm-force-cleanup.sh
└── examples/             # Example configurations
    ├── deploy-controller.yaml
    ├── storm-cluster.yaml
    └── test-*.yaml
```

## Quick Start

### 1. Deploy Storm Cluster with Helm

```bash
helm install storm-kubernetes ./charts/storm-kubernetes \
  -n storm-system --create-namespace \
  --set crd.install=true \
  --set controller.enabled=false \
  --set supervisor.readinessProbe.enabled=false \
  --set supervisor.livenessProbe.enabled=false
```

### 2. Build and Deploy Controller

```bash
# Build controller image
cd src
docker build -t storm-controller:latest .

# Deploy controller
kubectl apply -f examples/deploy-controller.yaml
```

### 3. Create StormCluster Reference

```bash
kubectl apply -f examples/storm-cluster.yaml
```

### 4. Deploy a Topology

```bash
# Using URL-based JAR
kubectl apply -f examples/test-url-topology.yaml

# Using container-based JAR
kubectl apply -f examples/test-container-topology.yaml
```

## Controller Features

### Version Management

The controller tracks topology versions and automatically handles updates:

```yaml
spec:
  topology:
    config:
      topology.version: "1.0.0"  # Change this to trigger update
```

When the version changes:
1. Controller kills the existing topology
2. Waits for complete removal from Storm
3. Deploys the new version
4. Updates the deployed version in status

### Container-based JAR Deployment

Deploy topologies with JAR files packaged in container images:

```yaml
spec:
  topology:
    jar:
      container:
        image: "your-registry/topology:latest"
        path: "/app/topology.jar"
        extractionMode: "job"
```

### Improved Deletion

The controller uses Storm CLI for reliable topology deletion, handling finalizers automatically.

## Development

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster
- kubectl configured

### Building

```bash
cd src
make generate  # Generate CRD code
make build     # Build binary
make docker-build IMG=your-repo/storm-controller:latest
```

### Running Locally

```bash
cd src
make install   # Install CRDs
make run      # Run controller locally
```

## Troubleshooting

### Cleanup Scripts

Use the provided cleanup scripts to remove Storm deployments:

```bash
# Normal cleanup
./scripts/storm-controller-cleanup.sh storm-system

# Force cleanup (if resources are stuck)
./scripts/storm-force-cleanup.sh storm-system
```

## Recent Improvements

- **CLI-based Deletion**: Replaced unreliable API deletion with Storm CLI
- **Version Tracking**: Added `deployedVersion` field to track running versions
- **Update Workflow**: Automatic topology updates on version change
- **Enhanced Logging**: Comprehensive logging of all operations
- **Container JAR Support**: Extract JARs from container images

## Future Enhancements

- **CRD-Only Mode**: Support deployment mode where only CRDs and controller are deployed via Helm, and the controller creates all Storm cluster resources (Nimbus, Supervisors, UI, ConfigMaps, Services). This would enable GitOps-friendly deployments where the entire Storm cluster is defined through StormCluster CRD resources.

## License

Apache License 2.0