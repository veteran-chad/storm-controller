#!/bin/bash

# Storm Force Cleanup Script - Aggressive version
# This script forcefully removes Storm namespace and CRDs

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if namespace argument is provided
if [ $# -eq 0 ]; then
    print_error "No namespace provided"
    echo "Usage: $0 <namespace>"
    echo "Example: $0 storm-system"
    exit 1
fi

NAMESPACE=$1

print_info "Starting FORCE cleanup for namespace: $NAMESPACE"
print_warn "This will aggressively delete resources!"

# Uninstall Helm releases
if command -v helm &> /dev/null; then
    print_info "Force uninstalling Helm releases..."
    helm list -n "$NAMESPACE" -q 2>/dev/null | xargs -r helm uninstall -n "$NAMESPACE" --no-hooks 2>/dev/null || true
fi

# Remove finalizers from all Storm CRs
print_info "Removing ALL finalizers from Storm resources..."
for resource in stormtopology stormcluster stormworkerpool; do
    kubectl get "$resource" -n "$NAMESPACE" -o name 2>/dev/null | while read -r item; do
        kubectl patch "$item" -n "$NAMESPACE" --type merge -p '{"metadata":{"finalizers":[]}}' 2>/dev/null || true
    done
done

# Force delete all Storm CRs
print_info "Force deleting all Storm custom resources..."
kubectl delete stormtopology,stormcluster,stormworkerpool --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true

# Delete all resources in namespace
print_info "Force deleting ALL resources in namespace..."
kubectl delete all --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
kubectl delete configmap,secret,pvc --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true

# Force delete namespace
print_info "Force deleting namespace..."
kubectl delete namespace "$NAMESPACE" --force --grace-period=0 2>/dev/null || true

# If namespace is stuck, patch it
if kubectl get namespace "$NAMESPACE" 2>/dev/null; then
    print_warn "Namespace stuck, removing finalizers..."
    kubectl patch namespace "$NAMESPACE" --type merge -p '{"metadata":{"finalizers":[]}}' 2>/dev/null || true
    kubectl patch namespace "$NAMESPACE" --type merge -p '{"spec":{"finalizers":[]}}' 2>/dev/null || true
fi

# Delete CRDs
print_info "Force deleting Storm CRDs..."
kubectl delete crd stormclusters.storm.apache.org stormtopologies.storm.apache.org stormworkerpools.storm.apache.org --force --grace-period=0 2>/dev/null || true

# Kill port-forwards
pkill -f "port-forward.*storm" 2>/dev/null || true
pkill -f "port-forward.*$NAMESPACE" 2>/dev/null || true

print_info "Force cleanup complete!"