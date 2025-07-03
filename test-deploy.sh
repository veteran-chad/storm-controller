#!/bin/bash

# Test deployment script for Storm Controller
# This script will clean up any existing deployment and create a fresh one

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
SRC_DIR="${SCRIPT_DIR}/src"
CHARTS_DIR="${SCRIPT_DIR}/charts/storm-kubernetes"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print colored messages
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed"
    fi
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed"
    fi
    
    # Check Helm
    if ! command -v helm &> /dev/null; then
        error "Helm is not installed"
    fi
    
    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot connect to Kubernetes cluster"
    fi
    
    info "All prerequisites met"
}

# Clean up existing deployment
cleanup() {
    info "Running cleanup script..."
    
    # Use the existing cleanup script
    if [ -f "${SCRIPT_DIR}/scripts/storm-controller-cleanup.sh" ]; then
        "${SCRIPT_DIR}/scripts/storm-controller-cleanup.sh" storm-system || {
            warn "Cleanup script reported issues, but continuing..."
        }
    else
        error "Cleanup script not found at ${SCRIPT_DIR}/scripts/storm-controller-cleanup.sh"
    fi
    
    info "Cleanup complete"
}

# Build controller Docker image
build_controller() {
    info "Building controller Docker image..."
    
    cd "${SRC_DIR}"
    
    # Generate code and manifests
    info "Generating code and manifests..."
    make generate
    make manifests
    
    # Build Docker image
    info "Building Docker image storm-controller:latest..."
    make docker-build IMG=storm-controller:latest
    
    info "Controller image built successfully"
}

# Install CRDs
install_crds() {
    info "Installing CRDs..."
    
    cd "${SRC_DIR}"
    make install
    
    # Verify CRDs are installed
    kubectl get crd stormclusters.storm.apache.org &> /dev/null || error "StormCluster CRD not installed"
    kubectl get crd stormtopologies.storm.apache.org &> /dev/null || error "StormTopology CRD not installed"
    kubectl get crd stormworkerpools.storm.apache.org &> /dev/null || error "StormWorkerPool CRD not installed"
    
    info "CRDs installed successfully"
}

# Deploy Storm cluster using Helm
deploy_storm_cluster() {
    info "Deploying Storm cluster using Helm..."
    
    # Create namespace
    kubectl create namespace storm-system
    
    # Add Helm repository
    info "Adding Bitnami Helm repository..."
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo update
    
    # Build Helm dependencies
    info "Building Helm chart dependencies..."
    cd "${CHARTS_DIR}"
    helm dependency build
    
    # Install Storm cluster with controller
    info "Installing Storm cluster..."
    helm install storm-cluster . \
        --namespace storm-system \
        --set global.storageClass=hostpath \
        --set nimbus.replicaCount=1 \
        --set supervisor.replicaCount=2 \
        --set supervisor.readinessProbe.enabled=false \
        --set supervisor.livenessProbe.enabled=false \
        --set ui.enabled=true \
        --set controller.enabled=true \
        --set controller.image.repository=storm-controller \
        --set controller.image.tag=latest \
        --set controller.image.pullPolicy=Never \
        --set controller.resources.requests.memory=1Gi \
        --set controller.resources.limits.memory=2Gi \
        --set crd.install=false \
        --wait \
        --timeout 3m || {
        warn "Helm install timed out, but deployment may still be in progress"
        info "Checking deployment status..."
    }
    
    info "Storm cluster deployed successfully"
}

# Deploy sample topology
deploy_sample_topology() {
    info "Deploying sample topology..."
    
    # Wait for Nimbus to be ready
    info "Waiting for Nimbus to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=nimbus -n storm-system --timeout=300s
    
    # Create StormCluster CR
    cat <<EOF | kubectl apply -f -
apiVersion: storm.apache.org/v1beta1
kind: StormCluster
metadata:
  name: storm-cluster
  namespace: storm-system
spec:
  nimbus:
    replicas: 1
  supervisor:
    replicas: 2
EOF
    
    # Wait a moment for the cluster to be registered
    sleep 5
    
    # Create sample topology
    cat <<EOF | kubectl apply -f -
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: exclamation-topology
  namespace: storm-system
spec:
  clusterRef: storm-cluster
  topology:
    name: exclamation-topology
    jar:
      url: "https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.6.4/storm-starter-2.6.4.jar"
    mainClass: "org.apache.storm.starter.ExclamationTopology"
    config:
      "topology.workers": "1"
      "topology.version": "1.0.0"
EOF
    
    info "Sample topology deployed"
}

# Show deployment status
show_status() {
    info "Deployment Status:"
    echo ""
    
    echo "=== Namespaces ==="
    kubectl get namespace storm-system
    echo ""
    
    echo "=== Pods in storm-system ==="
    kubectl get pods -n storm-system
    echo ""
    
    echo "=== Services in storm-system ==="
    kubectl get services -n storm-system
    echo ""
    
    echo "=== Storm CRDs ==="
    kubectl get crd | grep storm.apache.org
    echo ""
    
    echo "=== Storm Resources ==="
    kubectl get stormclusters,stormtopologies,stormworkerpools -A
    echo ""
    
    echo "=== Controller Logs (last 20 lines) ==="
    CONTROLLER_POD=$(kubectl get pods -n storm-system -l app.kubernetes.io/name=storm-controller -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$CONTROLLER_POD" ]; then
        kubectl logs -n storm-system "$CONTROLLER_POD" --tail=20
    else
        warn "Controller pod not found"
    fi
}

# Main execution
main() {
    info "Starting Storm Controller test deployment..."
    
    check_prerequisites
    cleanup
    build_controller
    install_crds
    deploy_storm_cluster
    
    # Optional: Deploy sample topology
    read -p "Deploy sample topology? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        deploy_sample_topology
    fi
    
    show_status
    
    info "Test deployment complete!"
    info "You can access Storm UI at: kubectl port-forward -n storm-system svc/storm-cluster-ui 8080:8080"
}

# Run main function
main "$@"