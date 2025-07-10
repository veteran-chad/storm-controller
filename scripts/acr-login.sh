#!/bin/bash
#
# Azure Container Registry Login Script
# 
# This script logs into Azure Container Registry using environment variables
# for both Docker and Helm CLI tools.
#
# Required environment variables:
#   - ACR_REGISTRY: The ACR registry URL (e.g., myregistry.azurecr.io)
#   - ACR_USERNAME: The ACR username (or service principal ID)
#   - ACR_PASSWORD: The ACR password (or service principal password)
#
# Usage:
#   export ACR_REGISTRY=myregistry.azurecr.io
#   export ACR_USERNAME=<username>
#   export ACR_PASSWORD=<password>
#   ./acr-login.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check for required environment variables
MISSING_VARS=()

if [ -z "$ACR_REGISTRY" ]; then
    MISSING_VARS+=("ACR_REGISTRY")
fi

if [ -z "$ACR_USERNAME" ]; then
    MISSING_VARS+=("ACR_USERNAME")
fi

if [ -z "$ACR_PASSWORD" ]; then
    MISSING_VARS+=("ACR_PASSWORD")
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
    echo "  export ACR_USERNAME=<your-username>"
    echo "  export ACR_PASSWORD=<your-password>"
    echo
    echo "For service principal authentication:"
    echo "  - ACR_USERNAME should be the service principal ID"
    echo "  - ACR_PASSWORD should be the service principal password"
    echo
    echo "For admin user authentication:"
    echo "  - ACR_USERNAME should be the registry name"
    echo "  - ACR_PASSWORD should be the admin password"
    echo
    exit 1
fi

# Validate ACR_REGISTRY format
if [[ ! "$ACR_REGISTRY" =~ \.azurecr\.io$ ]]; then
    print_error "ACR_REGISTRY should end with .azurecr.io (e.g., myregistry.azurecr.io)"
    echo "Current value: $ACR_REGISTRY"
    exit 1
fi

# Check if Docker is installed
if ! command_exists docker; then
    print_error "Docker is not installed or not in PATH"
    echo "Please install Docker: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check if Helm is installed
if ! command_exists helm; then
    print_error "Helm is not installed or not in PATH"
    echo "Please install Helm: https://helm.sh/docs/intro/install/"
    exit 1
fi

print_info "Starting Azure Container Registry login process..."
echo

# Docker login
print_info "Logging into Docker..."
echo "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" \
    --username "$ACR_USERNAME" \
    --password-stdin

if [ $? -eq 0 ]; then
    print_success "Docker login successful!"
else
    print_error "Docker login failed"
    exit 1
fi

echo

# Helm registry login
print_info "Logging into Helm registry..."
echo "$ACR_PASSWORD" | helm registry login "$ACR_REGISTRY" \
    --username "$ACR_USERNAME" \
    --password-stdin

if [ $? -eq 0 ]; then
    print_success "Helm registry login successful!"
else
    print_error "Helm registry login failed"
    exit 1
fi

echo
print_success "Successfully logged into Azure Container Registry!"
echo
echo "Registry: $ACR_REGISTRY"
echo "Username: $ACR_USERNAME"
echo
echo "You can now:"
echo "  - Push/pull Docker images: docker push $ACR_REGISTRY/myimage:tag"
echo "  - Push/pull Helm charts: helm push mychart.tgz oci://$ACR_REGISTRY/helm"
echo