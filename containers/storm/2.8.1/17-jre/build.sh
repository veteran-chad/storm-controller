#!/bin/bash

set -e

# Function to display usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build and tag Apache Storm Docker image

OPTIONS:
    -v, --version VERSION    Storm version (required, e.g., 2.8.1)
    -t, --tag TAG           Base image tag (required, e.g., 17-jre)
    -b, --build-id ID       Optional build ID for additional tag
    -r, --registry REGISTRY Docker registry (default: docker.io)
    -n, --no-cache          Build without cache
    -h, --help              Display this help message

EXAMPLES:
    # Basic build
    $0 --version 2.8.1 --tag 17-jre

    # Build with build ID
    $0 --version 2.8.1 --tag 17-jre --build-id 20250725.1

    # Build with custom registry
    $0 --version 2.8.1 --tag 17-jre --registry myregistry.io

    # Build without cache
    $0 --version 2.8.1 --tag 17-jre --no-cache

EOF
    exit 1
}

# Initialize variables
VERSION=""
TAG=""
BUILD_ID=""
REGISTRY="docker.io"
NO_CACHE=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        -b|--build-id)
            BUILD_ID="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -n|--no-cache)
            NO_CACHE="--no-cache"
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required arguments
if [ -z "$VERSION" ] || [ -z "$TAG" ]; then
    echo "Error: Version and tag are required"
    usage
fi

# Remove trailing slash from registry if present
REGISTRY="${REGISTRY%/}"

# Construct image names
BASE_IMAGE="${REGISTRY}/storm:${VERSION}-${TAG}"
BUILD_IMAGE="${REGISTRY}/storm:${VERSION}-${TAG}-${BUILD_ID}"

# Display build information
echo "=========================================="
echo "Apache Storm Docker Image Build"
echo "=========================================="
echo "Version:      $VERSION"
echo "Tag:          $TAG"
echo "Build ID:     ${BUILD_ID:-none}"
echo "Registry:     $REGISTRY"
echo "Base Image:   $BASE_IMAGE"
if [ -n "$BUILD_ID" ]; then
    echo "Build Image:  $BUILD_IMAGE"
fi
echo "No Cache:     ${NO_CACHE:-false}"
echo "=========================================="
echo

# Confirm build
read -p "Proceed with build? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Build cancelled"
    exit 1
fi

# Build the Docker image
echo "Building Docker image..."
docker build \
    --build-arg VERSION="$VERSION" \
    --build-arg TAG="$TAG" \
    --build-arg SERVICE=storm \
    --build-arg ENVIRONMENT=production \
    --tag "$BASE_IMAGE" \
    $NO_CACHE \
    .

# Check if build was successful
if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo "Tagged as: $BASE_IMAGE"
    
    # Tag with build ID if provided
    if [ -n "$BUILD_ID" ]; then
        docker tag "$BASE_IMAGE" "$BUILD_IMAGE"
        echo "Also tagged as: $BUILD_IMAGE"
    fi
    
    echo
    echo "To push the image(s):"
    echo "  docker push $BASE_IMAGE"
    if [ -n "$BUILD_ID" ]; then
        echo "  docker push $BUILD_IMAGE"
    fi
else
    echo "Build failed!"
    exit 1
fi