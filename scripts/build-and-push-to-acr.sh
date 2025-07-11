#!/bin/bash
#
# Build and Push Storm Controller to Azure Container Registry
# 
# This script builds the storm controller and example topology images
# and pushes them to Azure Container Registry.
#
# Required environment variables:
#   - ACR_REGISTRY: The ACR registry URL (e.g., myregistry.azurecr.io)
#   - ACR_USERNAME: The ACR username (for login)
#   - ACR_PASSWORD: The ACR password (for login)
#
# Optional parameters:
#   - STORM_VERSION: Storm version to use (default: 2.8.1)
#   - PUSH_IMAGES: Whether to push images (default: true, set to false for build only)
#
# Usage:
#   ./build-and-push-to-acr.sh
#   PUSH_IMAGES=false ./build-and-push-to-acr.sh  # Build only, don't push

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
STORM_VERSION="${STORM_VERSION:-2.8.1}"
PUSH_IMAGES="${PUSH_IMAGES:-true}"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

# Check for required environment variables
MISSING_VARS=()

if [ -z "$ACR_REGISTRY" ]; then
    MISSING_VARS+=("ACR_REGISTRY")
fi

if [ "$PUSH_IMAGES" == "true" ]; then
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
    if [ "$PUSH_IMAGES" == "true" ]; then
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

print_info "Build and Push Configuration:"
echo "  Registry:      $ACR_REGISTRY"
echo "  Storm Version: $STORM_VERSION"
echo "  Push Images:   $PUSH_IMAGES"
echo "  Project Root:  $PROJECT_ROOT"
echo

# Login to ACR if pushing
if [ "$PUSH_IMAGES" == "true" ]; then
    print_step "Logging into Azure Container Registry..."
    echo "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" \
        --username "$ACR_USERNAME" \
        --password-stdin
    
    if [ $? -eq 0 ]; then
        print_success "ACR login successful!"
    else
        print_error "ACR login failed"
        exit 1
    fi
    echo
fi

# Build storm controller
print_step "Building Storm Controller image..."
cd "$PROJECT_ROOT/src"

docker build -t storm-controller:latest \
    --build-arg STORM_VERSION=$STORM_VERSION \
    .

if [ $? -eq 0 ]; then
    print_success "Storm Controller built successfully!"
else
    print_error "Storm Controller build failed"
    exit 1
fi

# Tag controller for ACR
CONTROLLER_TAG="${ACR_REGISTRY}/rts/storm-controller:latest"
CONTROLLER_VERSION_TAG="${ACR_REGISTRY}/rts/storm-controller:${STORM_VERSION}"

print_info "Tagging controller image as $CONTROLLER_TAG"
docker tag storm-controller:latest "$CONTROLLER_TAG"
docker tag storm-controller:latest "$CONTROLLER_VERSION_TAG"

# Build storm topology example
print_step "Building Storm Topology Example image..."
cd "$PROJECT_ROOT/examples/storm-topology-example"

docker build -t storm-topology-example:latest .

if [ $? -eq 0 ]; then
    print_success "Storm Topology Example built successfully!"
else
    print_error "Storm Topology Example build failed"
    exit 1
fi

# Tag topology example for ACR
TOPOLOGY_TAG="${ACR_REGISTRY}/rts/storm-topology-example:latest"
TOPOLOGY_VERSION_TAG="${ACR_REGISTRY}/rts/storm-topology-example:${STORM_VERSION}"

print_info "Tagging topology example as $TOPOLOGY_TAG"
docker tag storm-topology-example:latest "$TOPOLOGY_TAG"
docker tag storm-topology-example:latest "$TOPOLOGY_VERSION_TAG"

# Push images if enabled
if [ "$PUSH_IMAGES" == "true" ]; then
    echo
    print_step "Pushing Storm Controller to ACR..."
    docker push "$CONTROLLER_TAG"
    docker push "$CONTROLLER_VERSION_TAG"
    
    if [ $? -eq 0 ]; then
        print_success "Storm Controller pushed successfully!"
    else
        print_error "Storm Controller push failed"
        exit 1
    fi
    
    echo
    print_step "Pushing Storm Topology Example to ACR..."
    docker push "$TOPOLOGY_TAG"
    docker push "$TOPOLOGY_VERSION_TAG"
    
    if [ $? -eq 0 ]; then
        print_success "Storm Topology Example pushed successfully!"
    else
        print_error "Storm Topology Example push failed"
        exit 1
    fi
fi

# Summary
echo
print_success "Build complete!"
echo
echo "Images built:"
echo "  - storm-controller:latest"
echo "  - storm-topology-example:latest"
echo

if [ "$PUSH_IMAGES" == "true" ]; then
    echo "Images pushed to ACR:"
    echo "  - $CONTROLLER_TAG"
    echo "  - $CONTROLLER_VERSION_TAG"
    echo "  - $TOPOLOGY_TAG"
    echo "  - $TOPOLOGY_VERSION_TAG"
    echo
    echo "To use these images in your deployments:"
    echo
    echo "1. Create image pull secret:"
    echo "   $SCRIPT_DIR/create-acr-pull-secret.sh <namespace>"
    echo
    echo "2. Update Helm values:"
    echo "   image:"
    echo "     repository: ${ACR_REGISTRY}/rts/storm-controller"
    echo "     tag: latest"
    echo "     pullSecrets:"
    echo "       - name: acr-pull-secret"
    echo
    echo "3. For topology CRDs:"
    echo "   jar:"
    echo "     container:"
    echo "       image: ${ACR_REGISTRY}/rts/storm-topology-example:latest"
    echo "       path: /storm-starter.jar"
    echo "       imagePullSecrets:"
    echo "         - name: acr-pull-secret"
else
    echo "Images were built but NOT pushed (PUSH_IMAGES=false)"
fi
echo