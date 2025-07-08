# Node Affinity and Tolerations Configuration

This document explains how to configure node selectors and tolerations for Storm components when deploying with the Helm chart.

## Overview

All Storm components (Nimbus, Supervisor, UI, Controller, and Zookeeper) support Kubernetes node selectors and tolerations. This allows you to:

1. **Schedule pods on specific nodes** using node selectors
2. **Allow pods to be scheduled on tainted nodes** using tolerations

## Configuration

### Example: Dedicated Storm Nodes

To deploy Storm on dedicated nodes that are labeled and tainted for Storm workloads:

1. **Label your nodes:**
   ```bash
   kubectl label nodes <node-name> node=storm
   ```

2. **Taint your nodes (optional):**
   ```bash
   kubectl taint nodes <node-name> type=storm:NoSchedule
   ```

3. **Use the provided values file:**
   ```bash
   helm install storm-cluster ./charts/storm-kubernetes \
     -f charts/storm-kubernetes/storm-node-affinity-values.yaml \
     --namespace storm-system --create-namespace
   ```

### Component Configuration

Each component can be configured independently:

```yaml
# Nimbus
nimbus:
  nodeSelector:
    node: storm
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"

# Supervisor
supervisor:
  nodeSelector:
    node: storm
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"

# UI
ui:
  nodeSelector:
    node: storm
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"

# Controller
controller:
  nodeSelector:
    node: storm
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"

# Zookeeper (Bitnami subchart)
zookeeper:
  nodeSelector:
    node: storm
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"
```

## Advanced Examples

### Multiple Node Selectors

```yaml
supervisor:
  nodeSelector:
    node: storm
    instance-type: compute-optimized
    zone: us-east-1a
```

### Multiple Tolerations

```yaml
supervisor:
  tolerations:
  - key: "type"
    operator: "Equal"
    value: "storm"
    effect: "NoSchedule"
  - key: "dedicated"
    operator: "Equal"
    value: "storm-workers"
    effect: "NoSchedule"
  - key: "spot-instance"
    operator: "Exists"
    effect: "NoSchedule"
```

### Different Configurations per Component

You might want different node configurations for different components:

```yaml
# Nimbus on stable nodes
nimbus:
  nodeSelector:
    node-type: stable
    
# Supervisors on spot instances
supervisor:
  nodeSelector:
    node-type: spot
  tolerations:
  - key: "spot-instance"
    operator: "Exists"
    effect: "NoSchedule"
    
# UI on general nodes
ui:
  nodeSelector: {}
  tolerations: []
```

## Verification

After deployment, verify that pods are scheduled on the correct nodes:

```bash
# Check pod node assignments
kubectl get pods -n storm-system -o wide

# Describe a pod to see node selector and tolerations
kubectl describe pod <pod-name> -n storm-system
```

## Troubleshooting

If pods are not being scheduled:

1. **Check node labels:**
   ```bash
   kubectl get nodes --show-labels
   ```

2. **Check node taints:**
   ```bash
   kubectl describe nodes | grep -A5 Taints
   ```

3. **Check pod events:**
   ```bash
   kubectl describe pod <pod-name> -n storm-system
   ```

Common issues:
- **No nodes match selector**: Ensure nodes have the required labels
- **Pod tolerations don't match taints**: Verify toleration keys, values, and effects match exactly
- **Insufficient resources**: Even with correct selectors/tolerations, nodes need available resources