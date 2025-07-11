#!/bin/bash

echo "=== Testing Global Settings in Storm Kubernetes Chart ==="
echo

# Function to test a condition
test_condition() {
    local test_name="$1"
    local condition="$2"
    if eval "$condition"; then
        echo "✓ $test_name"
    else
        echo "✗ $test_name"
        exit 1
    fi
}

# Test 1: Global image registry override
echo "Test 1: Global Image Registry Override"
OUTPUT=$(helm template test-storm . -f test-global-values.yaml | grep "image:" | head -5)
test_condition "Images use global registry" "echo '$OUTPUT' | grep -q 'test-registry.example.com'"
echo "$OUTPUT" | head -3
echo

# Test 2: Global image pull secrets
echo "Test 2: Global Image Pull Secrets"
OUTPUT=$(helm template test-storm . -f test-global-values.yaml | grep -A3 "imagePullSecrets:" | head -10)
test_condition "Pull secret 1 is present" "echo '$OUTPUT' | grep -q 'test-pull-secret-1'"
test_condition "Pull secret 2 is present" "echo '$OUTPUT' | grep -q 'test-pull-secret-2'"
echo "$OUTPUT"
echo

# Test 3: Global storage class
echo "Test 3: Global Storage Class"
OUTPUT=$(helm template test-storm . -f test-global-values.yaml | grep "storageClassName:")
test_condition "Storage class is set" "echo '$OUTPUT' | grep -q 'test-storage-class'"
echo "$OUTPUT"
echo

# Test 4: Test with empty global settings (defaults)
echo "Test 4: Default Behavior (no global overrides)"
OUTPUT=$(helm template test-storm . -f storm-local-values.yaml | grep "image:" | grep -E "storm:|zookeeper:" | head -3)
test_condition "Default images don't have registry prefix" "echo '$OUTPUT' | grep -q 'storm:2.8.1'"
echo "$OUTPUT"
echo

# Test 5: Global registry behavior (Bitnami pattern)
echo "Test 5: Global Registry Behavior"
cat > test-component-override.yaml << EOF
global:
  imageRegistry: "global-registry.example.com"

nimbus:
  enabled: true
  image:
    registry: "component-registry.example.com"
    repository: storm
    tag: 2.8.1
    
ui:
  enabled: true
  image:
    repository: storm
    tag: 2.8.1
EOF

OUTPUT=$(helm template test-storm . -f test-component-override.yaml | grep "image:" | grep storm | head -2)
# Note: Bitnami common gives precedence to global.imageRegistry over component registries
test_condition "Global registry takes precedence (Bitnami pattern)" "echo '$OUTPUT' | grep -q 'global-registry.example.com'"
echo "$OUTPUT"
echo

# Test 6: Namespace handling
echo "Test 6: Namespace Handling"
OUTPUT=$(helm template test-storm . -f test-global-values.yaml --namespace custom-ns | grep "namespace:" | head -3)
test_condition "Namespace is applied" "echo '$OUTPUT' | grep -q 'custom-ns'"
echo "$OUTPUT"
echo

# Test 7: Common labels and annotations
echo "Test 7: Common Labels and Annotations"
cat > test-common-labels.yaml << EOF
commonLabels:
  team: platform
  environment: test
  
commonAnnotations:
  version: "1.0.0"
  managed-by: "helm"

nimbus:
  enabled: true
EOF

OUTPUT=$(helm template test-storm . -f test-common-labels.yaml | grep -A10 "test-storm-storm-kubernetes" | grep -A10 "labels:")
test_condition "Common labels are applied" "echo '$OUTPUT' | grep -q 'team: platform'"
test_condition "Environment label is applied" "echo '$OUTPUT' | grep -q 'environment: test'"
echo "$OUTPUT"
echo

echo "=== All tests passed! ==="