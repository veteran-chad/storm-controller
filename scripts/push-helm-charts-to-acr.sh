#!/bin/bash
#
# Push Helm Charts to Azure Container Registry as OCI Artifacts
# 
# This script packages and pushes all Helm charts to Azure Container Registry
# using the OCI registry format.
#
# Required environment variables:
#   - ACR_REGISTRY: The ACR registry URL (e.g., myregistry.azurecr.io)
#   - ACR_USERNAME: The ACR username (for login)
#   - ACR_PASSWORD: The ACR password (for login)
#
# Optional parameters:
#   - CHART_VERSION: Version to use for charts (default: 0.1.0)
#   - PUSH_CHARTS: Whether to push charts (default: true, set to false for package only)
#
# Usage:
#   ./push-helm-charts-to-acr.sh
#   PUSH_CHARTS=false ./push-helm-charts-to-acr.sh  # Package only, don't push
#   CHART_VERSION=1.0.0 ./push-helm-charts-to-acr.sh  # Use specific version

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

# Default values
CHART_VERSION="${CHART_VERSION:-0.1.0}"
PUSH_CHARTS="${PUSH_CHARTS:-true}"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
CHARTS_DIR="$PROJECT_ROOT/charts"
PACKAGE_DIR="$PROJECT_ROOT/.helm-packages"

# List of charts to package and push
CHARTS=(
    "storm-shared"
    "storm-operator"
    "storm-kubernetes"
    "storm-crd-cluster"
)

# Check for required environment variables
MISSING_VARS=()

if [ -z "$ACR_REGISTRY" ]; then
    MISSING_VARS+=("ACR_REGISTRY")
fi

if [ "$PUSH_CHARTS" == "true" ]; then
    if [ -z "$ACR_USERNAME" ]; then
        MISSING_VARS+=("ACR_USERNAME")
    fi
    
    if [ -z "$ACR_PASSWORD" ]; then
        MISSING_VARS+=("ACR_PASSWORD")
    fi
fi

# If any variables are missing, print error and exit
if [ ${#MISSING_VARS[@]} -ne 0 ]; then
    print_error "Missing required environment variables:"
    echo
    for var in "${MISSING_VARS[@]}"; do
        echo "  - $var"
    done
    echo
    echo "Please set the missing environment variables:"
    echo
    echo "  export ACR_REGISTRY=myregistry.azurecr.io"
    if [ "$PUSH_CHARTS" == "true" ]; then
        echo "  export ACR_USERNAME=<your-username>"
        echo "  export ACR_PASSWORD=<your-password>"
    fi
    echo
    exit 1
fi

# Validate ACR_REGISTRY format
if [[ ! "$ACR_REGISTRY" =~ \.azurecr\.io$ ]]; then
    print_error "ACR_REGISTRY should end with .azurecr.io (e.g., myregistry.azurecr.io)"
    echo "Current value: $ACR_REGISTRY"
    exit 1
fi

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    print_error "helm is not installed or not in PATH"
    echo "Please install helm: https://helm.sh/docs/intro/install/"
    exit 1
fi

print_info "Helm Chart Push Configuration:"
echo "  Registry:      $ACR_REGISTRY"
echo "  Chart Version: $CHART_VERSION"
echo "  Push Charts:   $PUSH_CHARTS"
echo "  Charts Dir:    $CHARTS_DIR"
echo

# Create package directory
mkdir -p "$PACKAGE_DIR"

# Login to ACR if pushing
if [ "$PUSH_CHARTS" == "true" ]; then
    print_step "Logging into Azure Container Registry for Helm..."
    echo "$ACR_PASSWORD" | helm registry login "$ACR_REGISTRY" \
        --username "$ACR_USERNAME" \
        --password-stdin
    
    if [ $? -eq 0 ]; then
        print_success "Helm ACR login successful!"
    else
        print_error "Helm ACR login failed"
        exit 1
    fi
    echo
fi

# Update chart versions
print_step "Updating chart versions to $CHART_VERSION..."
for chart in "${CHARTS[@]}"; do
    CHART_PATH="$CHARTS_DIR/$chart/Chart.yaml"
    if [ -f "$CHART_PATH" ]; then
        # Backup original
        cp "$CHART_PATH" "$CHART_PATH.bak"
        
        # Update version
        sed -i "s/^version: .*/version: $CHART_VERSION/" "$CHART_PATH"
        
        # Update storm-shared dependency version in dependent charts
        if [[ "$chart" != "storm-shared" ]]; then
            sed -i "/name: storm-shared/,/version:/ s/version: .*/version: \"$CHART_VERSION\"/" "$CHART_PATH"
        fi
        
        print_info "Updated $chart to version $CHART_VERSION"
    else
        print_error "Chart $chart not found at $CHART_PATH"
        exit 1
    fi
done

# Package storm-shared first (dependency for others)
print_step "Packaging storm-shared chart..."
helm package "$CHARTS_DIR/storm-shared" --destination "$PACKAGE_DIR"

if [ $? -eq 0 ]; then
    print_success "storm-shared packaged successfully!"
else
    print_error "storm-shared packaging failed"
    exit 1
fi

# Push storm-shared first if enabled (so other charts can find it)
if [ "$PUSH_CHARTS" == "true" ]; then
    print_info "Pushing storm-shared to ACR..."
    SHARED_PACKAGE="$PACKAGE_DIR/storm-shared-${CHART_VERSION}.tgz"
    helm push "$SHARED_PACKAGE" "oci://${ACR_REGISTRY}/helm/rts"
    
    if [ $? -eq 0 ]; then
        print_success "storm-shared pushed successfully!"
    else
        print_error "storm-shared push failed"
        exit 1
    fi
fi

# Update dependent charts to use ACR repository
print_step "Updating dependent charts to use ACR repository..."
for chart in storm-operator storm-kubernetes storm-crd-cluster; do
    CHART_PATH="$CHARTS_DIR/$chart/Chart.yaml"
    # Update repository to ACR
    sed -i "/name: storm-shared/,/repository:/ s|repository: .*|repository: \"oci://${ACR_REGISTRY}/helm/rts\"|" "$CHART_PATH"
done

# Build dependencies for remaining charts
print_step "Building dependencies for remaining charts..."
for chart in storm-operator storm-kubernetes storm-crd-cluster; do
    print_info "Building dependencies for $chart..."
    cd "$CHARTS_DIR/$chart"
    helm dependency update
    cd - > /dev/null
done

# Package remaining charts
print_step "Packaging remaining charts..."
for chart in storm-operator storm-kubernetes storm-crd-cluster; do
    print_info "Packaging $chart..."
    helm package "$CHARTS_DIR/$chart" --destination "$PACKAGE_DIR"
    
    if [ $? -eq 0 ]; then
        print_success "$chart packaged successfully!"
    else
        print_error "$chart packaging failed"
        exit 1
    fi
done

# Push remaining charts if enabled
if [ "$PUSH_CHARTS" == "true" ]; then
    echo
    print_step "Pushing remaining charts to ACR..."
    
    for chart in storm-operator storm-kubernetes storm-crd-cluster; do
        print_info "Pushing $chart to ACR..."
        CHART_PACKAGE="$PACKAGE_DIR/${chart}-${CHART_VERSION}.tgz"
        helm push "$CHART_PACKAGE" "oci://${ACR_REGISTRY}/helm/rts"
        
        if [ $? -eq 0 ]; then
            print_success "$chart pushed successfully!"
        else
            print_error "$chart push failed"
            exit 1
        fi
    done
fi

# Restore original Chart.yaml files
print_step "Restoring original Chart.yaml files..."
for chart in "${CHARTS[@]}"; do
    CHART_PATH="$CHARTS_DIR/$chart/Chart.yaml"
    if [ -f "$CHART_PATH.bak" ]; then
        mv "$CHART_PATH.bak" "$CHART_PATH"
    fi
done

# Summary
echo
print_success "Helm chart processing complete!"
echo
echo "Charts packaged in: $PACKAGE_DIR"
ls -la "$PACKAGE_DIR"/*.tgz
echo

if [ "$PUSH_CHARTS" == "true" ]; then
    echo "Charts pushed to ACR:"
    for chart in "${CHARTS[@]}"; do
        echo "  - oci://${ACR_REGISTRY}/helm/rts/${chart}:${CHART_VERSION}"
    done
    echo
    echo "To use these charts:"
    echo
    echo "1. Install Storm Operator:"
    echo "   helm install storm-operator oci://${ACR_REGISTRY}/helm/rts/storm-operator \\"
    echo "     --version ${CHART_VERSION} \\"
    echo "     --namespace storm-operator --create-namespace \\"
    echo "     --set image.repository=${ACR_REGISTRY}/rts/storm-controller \\"
    echo "     --set imagePullSecrets[0].name=acr-pull-secret"
    echo
    echo "2. Deploy Storm Cluster using CRDs:"
    echo "   helm install my-cluster oci://${ACR_REGISTRY}/helm/rts/storm-crd-cluster \\"
    echo "     --version ${CHART_VERSION} \\"
    echo "     --namespace storm-prod --create-namespace"
    echo
    echo "3. Or deploy Storm using traditional Helm:"
    echo "   helm install storm oci://${ACR_REGISTRY}/helm/rts/storm-kubernetes \\"
    echo "     --version ${CHART_VERSION} \\"
    echo "     --namespace storm-prod --create-namespace"
else
    echo "Charts were packaged but NOT pushed (PUSH_CHARTS=false)"
fi
echo