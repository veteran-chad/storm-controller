#!/bin/bash
#
# Deploy Storm Controller and Cluster from ACR
# This script deploys the storm-operator and storm cluster using Helm charts from ACR

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}SUCCESS: $1${NC}"
}

print_info() {
    echo -e "${YELLOW}INFO: $1${NC}"
}

print_step() {
    echo -e "${BLUE}===> $1${NC}"
}

# Configuration
NAMESPACE="storm-system"
ACR_REGISTRY="${ACR_REGISTRY:-hdscmnrtspsdevscuscr.azurecr.io}"
CHART_VERSION="0.1.0"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Check if ACR_REGISTRY is set
if [ -z "$ACR_REGISTRY" ]; then
    print_error "ACR_REGISTRY environment variable is not set"
    exit 1
fi

print_info "Storm Deployment Configuration:"
echo "  Namespace:     $NAMESPACE"
echo "  ACR Registry:  $ACR_REGISTRY"
echo "  Chart Version: $CHART_VERSION"
echo

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    print_error "Namespace $NAMESPACE does not exist"
    echo "Please run: ./scripts/create-acr-pull-secret.sh $NAMESPACE"
    exit 1
fi

# Check if ACR pull secret exists
if ! kubectl get secret acr-pull-secret -n "$NAMESPACE" &>/dev/null; then
    print_error "ACR pull secret not found in namespace $NAMESPACE"
    echo "Please run: ./scripts/create-acr-pull-secret.sh $NAMESPACE"
    exit 1
fi

# Deploy Storm Operator
print_step "Deploying Storm Operator from ACR..."
helm upgrade --install storm-operator \
    "oci://${ACR_REGISTRY}/helm/rts/storm-operator" \
    --version "$CHART_VERSION" \
    --namespace "$NAMESPACE" \
    --values "$SCRIPT_DIR/storm-operator-values.yaml" \
    --wait

if [ $? -eq 0 ]; then
    print_success "Storm Operator deployed successfully!"
else
    print_error "Storm Operator deployment failed"
    exit 1
fi

# Wait for operator to be ready
print_info "Waiting for Storm Operator to be ready..."
kubectl wait --for=condition=available --timeout=300s \
    deployment/storm-operator-operator -n "$NAMESPACE"

# Deploy Storm Cluster using CRDs
print_step "Deploying Storm Cluster using CRDs..."
helm upgrade --install rts-storm-cluster \
    "oci://${ACR_REGISTRY}/helm/rts/storm-crd-cluster" \
    --version "$CHART_VERSION" \
    --namespace "$NAMESPACE" \
    --values "$SCRIPT_DIR/storm-crd-cluster-values.yaml" \
    --wait

if [ $? -eq 0 ]; then
    print_success "Storm Cluster deployed successfully!"
else
    print_error "Storm Cluster deployment failed"
    exit 1
fi

# Wait for cluster to be ready
print_info "Waiting for Storm Cluster to be ready..."
echo "This may take a few minutes..."

# Wait for StormCluster CRD to be ready
kubectl wait --for=condition=Ready --timeout=300s \
    stormcluster/rts-storm-cluster -n "$NAMESPACE" || true

# Check deployment status
print_step "Checking deployment status..."
echo
echo "Storm Operator:"
kubectl get deployment storm-operator-operator -n "$NAMESPACE"
echo
echo "Storm Cluster:"
kubectl get stormcluster -n "$NAMESPACE"
echo
echo "Pods:"
kubectl get pods -n "$NAMESPACE"
echo
echo "Services:"
kubectl get svc -n "$NAMESPACE"

# Port forward information
echo
print_success "Deployment complete!"
echo
print_info "To access Storm UI:"
echo "  kubectl port-forward -n $NAMESPACE svc/rts-storm-cluster-ui 8080:8080"
echo "  Then open: http://localhost:8080"
echo
print_info "To deploy a topology:"
echo "  kubectl apply -f $SCRIPT_DIR/wordcount-topology.yaml"
echo
print_info "To check topology status:"
echo "  kubectl get stormtopology -n $NAMESPACE"
echo
print_info "To view logs:"
echo "  # Controller logs"
echo "  kubectl logs -n $NAMESPACE deployment/storm-operator-operator -f"
echo "  # Nimbus logs"
echo "  kubectl logs -n $NAMESPACE statefulset/rts-storm-cluster-nimbus -f"
echo