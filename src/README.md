# Storm Kubernetes Controller

The Storm Kubernetes Controller manages Apache Storm deployments on Kubernetes using Custom Resource Definitions (CRDs).

## Features

- **Declarative Topology Management**: Deploy and manage Storm topologies using Kubernetes resources
- **Auto-scaling**: Automatic scaling of worker pools based on metrics
- **Zero-downtime Updates**: Support for rebalancing topologies without downtime
- **Namespace-scoped**: Each controller instance manages a single Storm cluster in a namespace

## Architecture

The controller manages three CRDs:

1. **StormCluster**: References an existing Storm cluster deployment
2. **StormTopology**: Defines a Storm topology to deploy
3. **StormWorkerPool**: Manages dedicated worker pools for topologies with HPA support

## Prerequisites

- Kubernetes 1.19+
- Storm cluster deployed (e.g., using the Storm Helm chart)
- Go 1.21+ (for development)

## Installation

### Using Pre-built Image

```bash
# Deploy CRDs
kubectl apply -f config/crd/

# Deploy controller
kubectl apply -f config/manager/
```

### Using Helm Chart

Enable the controller in your Storm Helm deployment:

```yaml
controller:
  enabled: true
  image:
    repository: your-repo/storm-controller
    tag: latest
```

## Usage

### 1. Create a StormCluster Reference

First, create a StormCluster resource that points to your Storm deployment:

```yaml
apiVersion: storm.apache.org/v1alpha1
kind: StormCluster
metadata:
  name: my-storm-cluster
  namespace: storm-system
spec:
  nimbusService: storm-nimbus
  nimbusPort: 6627
  uiService: storm-ui
  uiPort: 8080
  restApiEnabled: true
```

### 2. Deploy a Topology

Create a StormTopology resource:

```yaml
apiVersion: storm.apache.org/v1alpha1
kind: StormTopology
metadata:
  name: wordcount
  namespace: storm-system
spec:
  jarUrl: https://example.com/storm-examples.jar
  mainClass: org.apache.storm.starter.WordCountTopology
  args: ["wordcount"]
  config:
    topology.workers: "3"
  updateStrategy: rebalance
```

### 3. Configure Auto-scaling (Optional)

Create a StormWorkerPool for advanced scaling:

```yaml
apiVersion: storm.apache.org/v1alpha1
kind: StormWorkerPool
metadata:
  name: wordcount-pool
  namespace: storm-system
spec:
  topologyName: wordcount
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
```

## Development

### Building

```bash
# Generate code
make generate

# Build binary
make build

# Build Docker image
make docker-build IMG=your-repo/storm-controller:latest

# Push image
make docker-push IMG=your-repo/storm-controller:latest
```

### Running Locally

```bash
# Install CRDs
make install

# Run controller locally
make run
```

### Testing

```bash
# Run unit tests
make test

# Run e2e tests (requires cluster)
make test-e2e
```

## Configuration

The controller accepts the following command-line flags:

- `--storm-cluster`: Name of the StormCluster resource to manage (default: "storm-cluster")
- `--storm-namespace`: Namespace containing the Storm cluster (default: "default")
- `--nimbus-host`: Override Nimbus host (optional)
- `--nimbus-port`: Nimbus Thrift port (default: 6627)
- `--ui-host`: Override Storm UI host (optional)
- `--ui-port`: Storm UI port (default: 8080)
- `--leader-elect`: Enable leader election for HA (default: false)

## Limitations

- JAR files must be accessible via HTTP(S) URLs
- No built-in authentication support (assumes open Storm cluster)
- Worker pool management is basic (full implementation pending)
- Thrift API submission not implemented (uses storm CLI)

## Troubleshooting

### Topology Submission Fails

1. Check controller logs:
   ```bash
   kubectl logs -n storm-system deployment/storm-controller
   ```

2. Verify JAR URL is accessible:
   ```bash
   curl -I <jar-url>
   ```

3. Check Storm cluster status:
   ```bash
   kubectl get stormcluster -n storm-system
   ```

### Workers Not Scaling

1. Check HPA status:
   ```bash
   kubectl get hpa -n storm-system
   ```

2. Verify metrics server is running:
   ```bash
   kubectl get deployment metrics-server -n kube-system
   ```

## Cleanup

To completely remove Storm from your cluster, use the cleanup scripts:

```bash
# Normal cleanup
../scripts/storm-controller-cleanup.sh storm-system

# Force cleanup (if resources are stuck)
../scripts/storm-force-cleanup.sh storm-system
```

These scripts will:
- Uninstall Helm releases
- Remove finalizers from custom resources
- Delete all Storm resources and namespace
- Remove Storm CRDs
- Clean up port-forwards

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

Apache License 2.0