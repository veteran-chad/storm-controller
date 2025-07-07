#!/bin/bash
# Sync generated CRDs to Helm chart

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SRC_CRD_DIR="${PROJECT_ROOT}/src/config/crd/bases"
HELM_CRD_DIR="${PROJECT_ROOT}/charts/storm-kubernetes/crds"

echo "Syncing CRDs from ${SRC_CRD_DIR} to ${HELM_CRD_DIR}"

# Copy generated CRDs to Helm chart
cp -v "${SRC_CRD_DIR}"/*.yaml "${HELM_CRD_DIR}/"

echo "CRDs synced successfully"