#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Generate deepcopy
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."