# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Kubebuilder-based Kubernetes controller for managing Apache Storm deployments. The controller manages Storm topologies through Custom Resource Definitions (CRDs) and provides features like version management, container-based JAR deployment, and reliable deletion mechanisms.

## Key Commands

### Development Commands

```bash
# Code generation (run before building)
cd src && make generate

# Build controller binary
cd src && make build

# Format and lint code
cd src && make fmt vet

# Run tests
cd src && make test

# Run controller locally (for development)
cd src && make install  # Install CRDs first
cd src && make run      # Run controller against cluster

# Build Docker image
cd src && make docker-build IMG=storm-controller:latest

# Deploy to cluster
cd src && make deploy IMG=storm-controller:latest
```

### Testing a Single Test
```bash
# Run specific test package
cd src && go test ./controllers/... -v

# Run with specific test name
cd src && go test ./controllers/... -run TestStormTopologyController -v
```

### Cleanup Scripts
```bash
# Clean up Storm resources in a namespace
bash scripts/storm-controller-cleanup.sh storm-system

# Force cleanup if resources are stuck
bash scripts/storm-force-cleanup.sh storm-system
```

## Important Testing Guidelines

- **ALWAYS use the `storm-system` namespace for testing** unless otherwise explicitly instructed
- **ALWAYS run cleanup scripts before deploying** to ensure a clean state:
  ```bash
  bash scripts/storm-controller-cleanup.sh storm-system
  ```
- **For local testing, always use the storm-local-values.yaml file**:
  ```bash
  # Using the deployment script
  bash scripts/deploy-local.sh
  
  # Or manually with Helm
  helm upgrade --install storm-cluster ./charts/storm-kubernetes \
    -f charts/storm-kubernetes/storm-local-values.yaml \
    --namespace storm-system --create-namespace
  ```
- Example topology for testing is available at `examples/wordcount-topology.yaml`

## Architecture Overview

### CRD Structure
The controller manages three Custom Resource Definitions:

1. **StormCluster** (`api/v1beta1/stormcluster_types.go`): References existing Storm cluster deployments
2. **StormTopology** (`api/v1beta1/stormtopology_types.go`): Defines Storm topologies to deploy with version management
3. **StormWorkerPool** (`api/v1beta1/stormworkerpool_types.go`): Manages worker pools with autoscaling capabilities

### Controller Logic
- **Main reconciliation loop**: `controllers/stormtopology_controller.go`
- **Storm client interactions**: `pkg/storm/client.go`
- **JAR management**: `pkg/jar/` directory handles different JAR sources (URL, container, S3)
- **Worker pool management**: `controllers/stormworkerpool_controller.go`

### Key Design Patterns

1. **Version-based Updates**: The controller tracks `topology.version` in the spec and automatically handles topology updates by killing the old version and deploying the new one.

2. **Finalizer Pattern**: Uses Kubernetes finalizers to ensure proper cleanup when topologies are deleted, preventing orphaned Storm processes.

3. **Status Management**: Each CRD has a comprehensive status section that tracks deployment state, errors, and operational metrics.

4. **Container JAR Extraction**: Supports extracting JAR files from container images using Kubernetes Jobs, enabling GitOps-friendly topology deployment.

## Deployment Architecture

### Helm Chart (`charts/storm-kubernetes/`)
- Deploys complete Storm cluster (Nimbus, Supervisors, UI, Zookeeper)
- Installs CRDs when `crd.install=true`
- Controller deployment controlled by `controller.enabled`

### Component Communication
- Controller → Nimbus: Uses Storm CLI in the container (based on storm:latest image)
- Controller → Kubernetes API: Manages CRDs and creates Jobs for JAR extraction
- Topology → Workers: Managed through StormWorkerPool resources

## Important Implementation Details

1. **Storm CLI Integration**: The controller uses Storm CLI commands instead of REST API for reliability, especially for topology deletion (see `pkg/storm/client.go`)

2. **JAR Cache**: Controller maintains a JAR cache at `/storm/jars/` to avoid repeated downloads

3. **Reconciliation Flow**:
   - Check if topology exists in Storm
   - Compare deployed version with desired version
   - If different, kill old topology and wait for removal
   - Deploy new version
   - Update status with deployment information

4. **Error Handling**: Comprehensive error handling with exponential backoff for retries, especially important for Storm API interactions

## Common Development Tasks

When modifying CRD definitions:
1. Edit types in `api/v1beta1/`
2. Run `make generate` to update DeepCopy methods
3. Run `make manifests` to update CRD YAML files
4. Run `make install` to apply CRD changes to cluster

When adding new controller logic:
1. Implement reconciliation logic in appropriate controller file
2. Add necessary RBAC permissions using `+kubebuilder:rbac` markers
3. Run `make manifests` to update RBAC configuration
4. Test locally with `make run`

## Debugging and Development Resources

- **Storm MCP Server**:
  - Use the storm-mcp-server for expertise on storm best practices and to assist troubleshooting and connecting to the cluster for debug

## Git Commit Guidelines

- **Never add coauthored statements to git commits**

## Chart Development Guidelines

- **When working on charts `/charts/storm-kubernetes`**:
  - Always reference the STYLE-GUIDE.md for editing helm and values configuration