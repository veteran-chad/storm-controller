#!/bin/bash

# Storm Controller Cleanup Script
# This script removes a Storm namespace, handles finalizers, and removes CRDs

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

print_info "Starting cleanup for namespace: $NAMESPACE"

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed or not in PATH"
    exit 1
fi

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    print_warn "Namespace $NAMESPACE does not exist"
else
    print_info "Found namespace $NAMESPACE"
    
    # Check for Helm releases in the namespace
    if command -v helm &> /dev/null; then
        print_info "Checking for Helm releases in namespace $NAMESPACE..."
        helm list -n "$NAMESPACE" 2>/dev/null | grep -v NAME | while read -r release _; do
            if [ -n "$release" ]; then
                print_info "Uninstalling Helm release: $release"
                helm uninstall "$release" -n "$NAMESPACE" --wait --timeout=60s 2>/dev/null || {
                    print_warn "Failed to uninstall Helm release $release, continuing..."
                }
            fi
        done
    else
        print_warn "Helm not found, skipping Helm cleanup"
    fi
    
    # Remove finalizers from StormTopology resources
    print_info "Removing finalizers from StormTopology resources..."
    kubectl get stormtopology -n "$NAMESPACE" -o name 2>/dev/null | while read -r topology; do
        print_info "Removing finalizer from $topology"
        kubectl patch "$topology" -n "$NAMESPACE" --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    
    # Remove finalizers from StormCluster resources
    print_info "Removing finalizers from StormCluster resources..."
    kubectl get stormcluster -n "$NAMESPACE" -o name 2>/dev/null | while read -r cluster; do
        print_info "Removing finalizer from $cluster"
        kubectl patch "$cluster" -n "$NAMESPACE" --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    
    # Remove finalizers from StormWorkerPool resources
    print_info "Removing finalizers from StormWorkerPool resources..."
    kubectl get stormworkerpool -n "$NAMESPACE" -o name 2>/dev/null | while read -r pool; do
        print_info "Removing finalizer from $pool"
        kubectl patch "$pool" -n "$NAMESPACE" --type json -p='[{"op": "remove", "path": "/metadata/finalizers"}]' 2>/dev/null || true
    done
    
    # Delete all Storm resources
    print_info "Deleting all Storm resources in namespace $NAMESPACE..."
    kubectl delete stormtopology --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    kubectl delete stormcluster --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    kubectl delete stormworkerpool --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    
    # Force delete any stuck resources
    print_info "Force deleting any stuck resources..."
    kubectl delete stormtopology --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
    kubectl delete stormcluster --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
    kubectl delete stormworkerpool --all -n "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
    
    # Delete other resources that might block namespace deletion
    print_info "Deleting all remaining resources in namespace..."
    kubectl delete all --all -n "$NAMESPACE" --timeout=60s 2>/dev/null || true
    kubectl delete configmap --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    kubectl delete secret --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    kubectl delete pvc --all -n "$NAMESPACE" --timeout=30s 2>/dev/null || true
    kubectl delete pv -l namespace="$NAMESPACE" --timeout=30s 2>/dev/null || true
    
    # Delete the namespace
    print_info "Deleting namespace $NAMESPACE..."
    kubectl delete namespace "$NAMESPACE" --timeout=60s 2>/dev/null || {
        print_warn "Normal deletion failed, attempting force delete..."
        kubectl delete namespace "$NAMESPACE" --force --grace-period=0 2>/dev/null || true
    }
    
    # Check if namespace is stuck in Terminating state
    if kubectl get namespace "$NAMESPACE" 2>/dev/null | grep -q Terminating; then
        print_warn "Namespace is stuck in Terminating state, attempting to clean up finalizers..."
        
        # Remove namespace finalizers
        kubectl get namespace "$NAMESPACE" -o json | \
            jq '.spec.finalizers = []' | \
            kubectl replace --raw "/api/v1/namespaces/$NAMESPACE/finalize" -f - 2>/dev/null || {
                print_error "Failed to remove namespace finalizers. You may need to manually clean up."
            }
    fi
fi

# Delete CRDs
print_info "Deleting Storm CRDs..."

CRDS=(
    "stormclusters.storm.apache.org"
    "stormtopologies.storm.apache.org"
    "stormworkerpools.storm.apache.org"
)

for crd in "${CRDS[@]}"; do
    if kubectl get crd "$crd" &> /dev/null; then
        print_info "Deleting CRD: $crd"
        kubectl delete crd "$crd" --timeout=30s 2>/dev/null || {
            print_warn "Normal deletion failed for $crd, attempting force delete..."
            kubectl delete crd "$crd" --force --grace-period=0 2>/dev/null || true
        }
    else
        print_info "CRD $crd not found, skipping..."
    fi
done

# Final verification
print_info "Verifying cleanup..."

# Check namespace
if kubectl get namespace "$NAMESPACE" &> /dev/null; then
    print_error "Namespace $NAMESPACE still exists!"
else
    print_info "Namespace $NAMESPACE successfully deleted"
fi

# Check CRDs
remaining_crds=0
for crd in "${CRDS[@]}"; do
    if kubectl get crd "$crd" &> /dev/null; then
        print_error "CRD $crd still exists!"
        ((remaining_crds++))
    fi
done

if [ $remaining_crds -eq 0 ]; then
    print_info "All Storm CRDs successfully deleted"
else
    print_error "$remaining_crds CRDs still remain"
fi

# Kill any remaining port-forwards
print_info "Cleaning up any Storm-related port-forwards..."
pkill -f "port-forward.*storm" 2>/dev/null || true

print_info "Cleanup complete!"

# Exit with error if cleanup was not complete
if kubectl get namespace "$NAMESPACE" &> /dev/null || [ $remaining_crds -gt 0 ]; then
    exit 1
fi

exit 0