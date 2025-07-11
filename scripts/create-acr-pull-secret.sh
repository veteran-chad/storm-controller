#!/bin/bash
#
# Create Azure Container Registry Pull Secret
# 
# This script creates a Kubernetes image pull secret for Azure Container Registry
# using environment variables. The secret can be used to pull private images.
#
# Required environment variables:
#   - ACR_REGISTRY: The ACR registry URL (e.g., myregistry.azurecr.io)
#   - ACR_USERNAME: The ACR username (or service principal ID)
#   - ACR_PASSWORD: The ACR password (or service principal password)
#
# Optional parameters:
#   - Namespace: Target namespace (default: current namespace)
#   - Secret name: Name of the secret (default: acr-pull-secret)
#
# Usage:
#   ./create-acr-pull-secret.sh [namespace] [secret-name]
#   ./create-acr-pull-secret.sh storm-system
#   ./create-acr-pull-secret.sh storm-system my-acr-secret

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

print_debug() {
    echo -e "${BLUE}DEBUG: $1${NC}"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Parse command line arguments
NAMESPACE=${1:-$(kubectl config view --minify -o jsonpath='{..namespace}')}
SECRET_NAME=${2:-acr-pull-secret}

# If namespace is still empty, use default
if [ -z "$NAMESPACE" ]; then
    NAMESPACE="default"
fi

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

# Check if kubectl is installed
if ! command_exists kubectl; then
    print_error "kubectl is not installed or not in PATH"
    echo "Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check if we can connect to the cluster
if ! kubectl cluster-info &>/dev/null; then
    print_error "Cannot connect to Kubernetes cluster"
    echo "Please ensure:"
    echo "  - kubectl is configured correctly"
    echo "  - KUBECONFIG is set if needed"
    echo "  - You have access to the cluster"
    exit 1
fi

print_info "Creating Azure Container Registry pull secret..."
echo
echo "Configuration:"
echo "  Registry:   $ACR_REGISTRY"
echo "  Username:   $ACR_USERNAME"
echo "  Namespace:  $NAMESPACE"
echo "  Secret:     $SECRET_NAME"
echo

# Check if namespace exists
if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    print_debug "Namespace '$NAMESPACE' exists"
else
    print_info "Namespace '$NAMESPACE' does not exist. Creating..."
    kubectl create namespace "$NAMESPACE"
    print_success "Namespace '$NAMESPACE' created"
fi

# Check if secret already exists
if kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" &>/dev/null; then
    print_info "Secret '$SECRET_NAME' already exists in namespace '$NAMESPACE'"
    read -p "Do you want to update it? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Cancelled by user"
        exit 0
    fi
    print_info "Deleting existing secret..."
    kubectl delete secret "$SECRET_NAME" -n "$NAMESPACE"
fi

# Create the secret
print_info "Creating image pull secret..."
kubectl create secret docker-registry "$SECRET_NAME" \
    --namespace="$NAMESPACE" \
    --docker-server="$ACR_REGISTRY" \
    --docker-username="$ACR_USERNAME" \
    --docker-password="$ACR_PASSWORD" \
    --docker-email="not-used@example.com"

if [ $? -eq 0 ]; then
    print_success "Image pull secret created successfully!"
else
    print_error "Failed to create image pull secret"
    exit 1
fi

# Verify the secret
echo
print_info "Verifying secret..."
if kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.type}' | grep -q "kubernetes.io/dockerconfigjson"; then
    print_success "Secret verified successfully"
else
    print_error "Secret verification failed"
    exit 1
fi

# Print usage instructions
echo
print_success "Image pull secret '$SECRET_NAME' created in namespace '$NAMESPACE'"
echo
echo "To use this secret in your deployments, add it to your pod spec:"
echo
cat << EOF
  spec:
    imagePullSecrets:
    - name: $SECRET_NAME
    containers:
    - name: my-container
      image: $ACR_REGISTRY/myimage:tag
EOF
echo
echo "For Helm deployments, you can use:"
echo
cat << EOF
  helm install my-release my-chart \\
    --set image.repository=$ACR_REGISTRY/myimage \\
    --set imagePullSecrets[0].name=$SECRET_NAME
EOF
echo
echo "To use with storm-controller charts:"
echo
cat << EOF
  helm install storm-operator ./charts/storm-operator \\
    --namespace $NAMESPACE \\
    --set image.repository=$ACR_REGISTRY/storm-controller \\
    --set imagePullSecrets[0].name=$SECRET_NAME
EOF
echo