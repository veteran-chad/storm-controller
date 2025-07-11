#!/bin/bash

# Deploy Storm cluster for local testing

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 Deploying Storm cluster for local testing..."

# Deploy with local values
helm upgrade --install storm-cluster \
  "${PROJECT_ROOT}/charts/storm-kubernetes" \
  -f "${PROJECT_ROOT}/charts/storm-kubernetes/storm-local-values.yaml" \
  --namespace storm-system \
  --create-namespace

echo "✅ Storm cluster deployment initiated"
echo ""
echo "To check status:"
echo "  kubectl get pods -n storm-system"
echo "  kubectl get stormcluster -n storm-system"
echo ""
echo "To access Storm UI:"
echo "  kubectl port-forward --namespace storm-system svc/storm-cluster-storm-kubernetes-ui 8080:8080 &"
echo "  open http://localhost:8080"