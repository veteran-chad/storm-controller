# Testing Strategy for Multi-Chart Architecture

## Overview

Comprehensive testing strategy for the new four-chart architecture ensuring all components work independently and together.

## Test Scenarios

### 1. Operator Installation Tests

#### Test 1.1: Install Operator with Default Zookeeper
```bash
# Install operator with default settings
helm install storm-operator ./charts/storm-operator \
  --namespace storm-operator --create-namespace

# Verify components
kubectl get pods -n storm-operator
kubectl get crds | grep storm
kubectl get cm -n storm-operator storm-operator-operator-config -o yaml

# Expected:
# - Storm operator pod running
# - 3 Zookeeper pods running
# - CRDs installed
# - ConfigMap with defaults created
```

#### Test 1.2: Install Operator with External Zookeeper
```bash
# Install operator without Zookeeper
helm install storm-operator ./charts/storm-operator \
  --namespace storm-operator --create-namespace \
  --set zookeeper.enabled=false \
  --set externalZookeeper.servers="{zk1:2181,zk2:2181,zk3:2181}"

# Verify operator config points to external ZK
kubectl get cm -n storm-operator storm-operator-operator-config -o yaml | grep servers
```

#### Test 1.3: Install Operator with Custom Registry
```bash
# Install with custom registry
helm install storm-operator ./charts/storm-operator \
  --namespace storm-operator --create-namespace \
  --set global.imageRegistry=myregistry.com \
  --set operator.defaults.storm.image.repository=custom/storm

# Verify images use custom registry
kubectl get deploy -n storm-operator -o yaml | grep image:
```

### 2. Cluster Deployment Tests

#### Test 2.1: Deploy Cluster with Default Zookeeper
```bash
# Deploy cluster using CRD
helm install prod-cluster ./charts/storm-crd-cluster \
  --namespace storm-prod --create-namespace

# Verify StormCluster resource
kubectl get stormcluster -n storm-prod
kubectl describe stormcluster -n storm-prod prod-cluster

# Expected:
# - StormCluster created with status "Pending"
# - Zookeeper servers point to operator namespace
# - Zookeeper root is "/storm/prod-cluster"
```

#### Test 2.2: Deploy Multiple Clusters
```bash
# Deploy first cluster
helm install prod-cluster ./charts/storm-crd-cluster \
  --namespace storm-prod --create-namespace \
  --set nameOverride=prod

# Deploy second cluster
helm install staging-cluster ./charts/storm-crd-cluster \
  --namespace storm-staging --create-namespace \
  --set nameOverride=staging

# Verify isolation
kubectl exec -n storm-operator storm-operator-zookeeper-0 -- \
  zkCli.sh ls /storm

# Expected output:
# [prod-cluster, staging-cluster]
```

#### Test 2.3: Deploy Cluster with External Zookeeper
```bash
# Deploy with external ZK
helm install ext-cluster ./charts/storm-crd-cluster \
  --namespace storm-external --create-namespace \
  --set zookeeper.external.enabled=true \
  --set zookeeper.external.servers="{zk1.example.com:2181}"

# Verify external ZK in StormCluster
kubectl get stormcluster -n storm-external ext-cluster -o yaml | grep -A2 zookeeper:
```

### 3. Configuration Tests

#### Test 3.1: Verify Config Merging
```bash
# Deploy cluster with custom config
cat > custom-values.yaml <<EOF
storm:
  config:
    topology.max.spout.pending: 1000
    topology.worker.childopts: "-Xmx2048m"
EOF

helm install custom-cluster ./charts/storm-crd-cluster \
  --namespace storm-custom --create-namespace \
  -f custom-values.yaml

# Wait for cluster to be created
sleep 30

# Check merged config in ConfigMap
kubectl get cm -n storm-custom custom-cluster-storm-config -o yaml

# Expected:
# - Default configs from operator
# - Overridden configs from values
# - storm.zookeeper.root: "/storm/custom-cluster"
```

#### Test 3.2: Global Registry Override
```bash
# Deploy with global registry
helm install global-test ./charts/storm-crd-cluster \
  --namespace storm-global --create-namespace \
  --set global.imageRegistry=gcr.io/myproject

# Verify image in created deployments
kubectl get deploy -n storm-global -o yaml | grep image:
# Should show: gcr.io/myproject/storm:2.8.1
```

### 4. Node Affinity Tests

#### Test 4.1: Deploy on Specific Nodes
```bash
# Label test nodes
kubectl label nodes worker-1 storm-node=true
kubectl taint nodes worker-1 dedicated=storm:NoSchedule

# Deploy with node affinity
helm install affinity-cluster ./charts/storm-crd-cluster \
  --namespace storm-affinity --create-namespace \
  --set nimbus.nodeSelector.storm-node=true \
  --set nimbus.tolerations[0].key=dedicated \
  --set nimbus.tolerations[0].value=storm \
  --set nimbus.tolerations[0].effect=NoSchedule

# Verify pod placement
kubectl get pods -n storm-affinity -o wide
```

### 5. Upgrade Tests

#### Test 5.1: Upgrade Operator
```bash
# Initial operator installation
helm install storm-operator ./charts/storm-operator \
  --namespace storm-operator --create-namespace \
  --version 0.1.0

# Deploy a cluster
helm install test-cluster ./charts/storm-crd-cluster \
  --namespace storm-test --create-namespace

# Upgrade operator
helm upgrade storm-operator ./charts/storm-operator \
  --namespace storm-operator \
  --version 0.2.0

# Verify cluster still running
kubectl get stormcluster -A
kubectl get pods -n storm-test
```

#### Test 5.2: Modify Cluster Configuration
```bash
# Initial deployment
helm install test-cluster ./charts/storm-crd-cluster \
  --namespace storm-test --create-namespace \
  --set supervisor.replicas=1

# Upgrade to scale up
helm upgrade test-cluster ./charts/storm-crd-cluster \
  --namespace storm-test \
  --set supervisor.replicas=3

# Verify scaling
kubectl get stormcluster -n storm-test test-cluster -o yaml | grep replicas
```

### 6. Integration Tests

#### Test 6.1: Full E2E Deployment
```bash
#!/bin/bash
# e2e-test.sh

set -e

NAMESPACE_OPERATOR="storm-operator-e2e"
NAMESPACE_CLUSTER="storm-cluster-e2e"

echo "=== Installing Operator ==="
helm install storm-operator ./charts/storm-operator \
  --namespace $NAMESPACE_OPERATOR --create-namespace \
  --wait --timeout 5m

echo "=== Verifying Operator ==="
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=controller \
  -n $NAMESPACE_OPERATOR --timeout=60s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=zookeeper \
  -n $NAMESPACE_OPERATOR --timeout=60s

echo "=== Deploying Storm Cluster ==="
helm install e2e-cluster ./charts/storm-crd-cluster \
  --namespace $NAMESPACE_CLUSTER --create-namespace \
  --set zookeeper.default.operatorNamespace=$NAMESPACE_OPERATOR

echo "=== Waiting for Cluster ==="
sleep 30
kubectl wait --for=condition=ready pod -l storm-component=nimbus \
  -n $NAMESPACE_CLUSTER --timeout=120s
kubectl wait --for=condition=ready pod -l storm-component=supervisor \
  -n $NAMESPACE_CLUSTER --timeout=120s

echo "=== Deploying Test Topology ==="
kubectl apply -f - <<EOF
apiVersion: storm.apache.org/v1beta1
kind: StormTopology
metadata:
  name: wordcount
  namespace: $NAMESPACE_CLUSTER
spec:
  clusterName: e2e-cluster
  topology:
    name: wordcount
    config:
      topology.workers: 1
    jar:
      url: "https://repo1.maven.org/maven2/org/apache/storm/storm-starter/2.8.1/storm-starter-2.8.1.jar"
    className: "org.apache.storm.starter.WordCountTopology"
    args: ["wordcount"]
EOF

echo "=== Verifying Topology ==="
sleep 30
kubectl get stormtopology -n $NAMESPACE_CLUSTER
kubectl logs -n $NAMESPACE_CLUSTER -l storm-component=nimbus --tail=50

echo "=== Cleanup ==="
kubectl delete namespace $NAMESPACE_CLUSTER
kubectl delete namespace $NAMESPACE_OPERATOR
```

### 7. Traditional Helm Deployment Test

#### Test 7.1: Deploy Storm without CRDs
```bash
# Refactored storm-kubernetes chart (no CRDs/controller)
helm install traditional-storm ./charts/storm-kubernetes \
  --namespace storm-traditional --create-namespace \
  --set-string externalZookeeper.servers="{zk1:2181,zk2:2181}"

# Verify traditional deployment
kubectl get deploy,sts,svc -n storm-traditional
```

## Test Matrix

| Test Category | storm-shared | storm-operator | storm-crd-cluster | storm-kubernetes |
|---------------|--------------|----------------|-------------------|------------------|
| Unit Tests    | ✓            | ✓              | ✓                 | ✓                |
| Helm Tests    | ✓            | ✓              | ✓                 | ✓                |
| Integration   | -            | ✓              | ✓                 | ✓                |
| E2E Tests     | -            | ✓              | ✓                 | -                |
| Upgrade Tests | -            | ✓              | ✓                 | ✓                |

## CI/CD Pipeline

```yaml
# .github/workflows/test-charts.yml
name: Test Charts

on:
  pull_request:
    paths:
      - 'charts/**'
      - 'src/**'

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Helm Lint
        run: |
          helm lint charts/storm-shared
          helm lint charts/storm-operator
          helm lint charts/storm-crd-cluster
          helm lint charts/storm-kubernetes

  test-operator:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Create k8s Kind Cluster
        uses: helm/kind-action@v1.5.0
      - name: Install Operator
        run: |
          helm install test-operator ./charts/storm-operator \
            --namespace storm-operator --create-namespace \
            --wait --timeout 5m
      - name: Run Tests
        run: |
          kubectl get pods -n storm-operator
          kubectl get crds | grep storm

  test-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Create k8s Kind Cluster
        uses: helm/kind-action@v1.5.0
      - name: Run E2E Tests
        run: |
          chmod +x ./tests/e2e-test.sh
          ./tests/e2e-test.sh
```

## Success Criteria

1. **Independent Operation**: Each chart can be installed/uninstalled independently
2. **Multi-tenancy**: Multiple Storm clusters can coexist with proper isolation
3. **Config Management**: Operator defaults are properly merged with cluster configs
4. **Upgrade Path**: Charts can be upgraded without disrupting running clusters
5. **Global Settings**: Global registry and image settings work across all charts
6. **Resource Isolation**: Each cluster uses its own Zookeeper path
7. **Traditional Support**: storm-kubernetes chart still works for non-CRD deployments