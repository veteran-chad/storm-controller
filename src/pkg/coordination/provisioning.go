/*
Copyright 2025 The Apache Software Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package coordination

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

// ProvisioningStrategy defines how worker pools should be provisioned
type ProvisioningStrategy string

const (
	// ProvisioningStrategyAutomatic automatically provisions worker pools based on topology requirements
	ProvisioningStrategyAutomatic ProvisioningStrategy = "Automatic"
	// ProvisioningStrategyManual requires manual worker pool creation
	ProvisioningStrategyManual ProvisioningStrategy = "Manual"
	// ProvisioningStrategyShared uses shared worker pools across topologies
	ProvisioningStrategyShared ProvisioningStrategy = "Shared"
)

// ProvisioningDecision represents a decision about worker pool provisioning
type ProvisioningDecision struct {
	Strategy          ProvisioningStrategy
	Action            ProvisioningAction
	WorkerPoolName    string
	RecommendedSpec   *stormv1beta1.StormWorkerPoolSpec
	Reason            string
	Message           string
	RequiresUserInput bool
	EstimatedCost     *ResourceCost
}

// ProvisioningAction defines what action should be taken
type ProvisioningAction string

const (
	ProvisioningActionCreate ProvisioningAction = "Create"
	ProvisioningActionUpdate ProvisioningAction = "Update"
	ProvisioningActionScale  ProvisioningAction = "Scale"
	ProvisioningActionUse    ProvisioningAction = "UseExisting"
	ProvisioningActionWait   ProvisioningAction = "Wait"
	ProvisioningActionNone   ProvisioningAction = "None"
)

// ResourceCost represents the estimated cost of a provisioning decision
type ResourceCost struct {
	CPUCores      float64
	MemoryGB      float64
	StorageGB     float64
	EstimatedCost string
}

// WorkerPoolProvisioner handles intelligent worker pool provisioning
type WorkerPoolProvisioner struct {
	client.Client
	Coordinator *ResourceCoordinator
	Scheme      *runtime.Scheme
}

// NewWorkerPoolProvisioner creates a new worker pool provisioner
func NewWorkerPoolProvisioner(client client.Client, coordinator *ResourceCoordinator, scheme *runtime.Scheme) *WorkerPoolProvisioner {
	return &WorkerPoolProvisioner{
		Client:      client,
		Coordinator: coordinator,
		Scheme:      scheme,
	}
}

// DetermineProvisioningStrategy analyzes a topology and determines the best provisioning strategy
func (wpp *WorkerPoolProvisioner) DetermineProvisioningStrategy(ctx context.Context, topology *stormv1beta1.StormTopology) (*ProvisioningDecision, error) {
	log := log.FromContext(ctx)

	// Check if topology has specific worker pool requirements
	requirements, err := wpp.Coordinator.CalculateWorkerPoolRequirements(ctx, topology)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate worker pool requirements: %w", err)
	}

	// Check existing worker pools for this topology
	existingPools, err := wpp.findExistingWorkerPools(ctx, topology)
	if err != nil {
		return nil, fmt.Errorf("failed to find existing worker pools: %w", err)
	}

	// Check cluster capacity
	clusterCapacity, err := wpp.Coordinator.GetClusterCapacity(ctx, topology.Spec.ClusterRef, topology.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster capacity: %w", err)
	}

	// Generate provisioning decision based on analysis
	decision := &ProvisioningDecision{
		WorkerPoolName: wpp.generateWorkerPoolName(topology),
		EstimatedCost:  wpp.calculateResourceCost(requirements),
	}

	// Analyze different strategies
	if len(existingPools) > 0 {
		// Check if we can use an existing worker pool
		suitablePool := wpp.findSuitablePool(existingPools, requirements)
		if suitablePool != nil {
			decision.Strategy = ProvisioningStrategyShared
			decision.Action = ProvisioningActionUse
			decision.WorkerPoolName = suitablePool.Name
			decision.Reason = "SuitablePoolFound"
			decision.Message = fmt.Sprintf("Using existing worker pool %s", suitablePool.Name)
			return decision, nil
		}
	}

	// Check if we need to create a new pool
	if clusterCapacity.AvailableSlots < requirements.MinWorkers {
		decision.Strategy = ProvisioningStrategyManual
		decision.Action = ProvisioningActionWait
		decision.Reason = "InsufficientCapacity"
		decision.Message = fmt.Sprintf("Insufficient cluster capacity. Available: %d, Required: %d", clusterCapacity.AvailableSlots, requirements.MinWorkers)
		decision.RequiresUserInput = true
		return decision, nil
	}

	// Determine if we should create a dedicated worker pool
	if requirements.MinWorkers > 1 || wpp.topologyRequiresDedicatedPool(topology) {
		decision.Strategy = ProvisioningStrategyAutomatic
		decision.Action = ProvisioningActionCreate
		decision.Reason = "DedicatedPoolRequired"
		decision.Message = fmt.Sprintf("Creating dedicated worker pool for topology %s", topology.Name)
		decision.RecommendedSpec = wpp.buildWorkerPoolSpec(topology, requirements)
	} else {
		// Use shared cluster resources
		decision.Strategy = ProvisioningStrategyShared
		decision.Action = ProvisioningActionNone
		decision.Reason = "UseClusterResources"
		decision.Message = "Using shared cluster resources, no dedicated worker pool needed"
	}

	log.Info("Determined provisioning strategy",
		"topology", topology.Name,
		"strategy", decision.Strategy,
		"action", decision.Action,
		"reason", decision.Reason)

	return decision, nil
}

// ProvisionWorkerPool provisions a worker pool based on the provisioning decision
func (wpp *WorkerPoolProvisioner) ProvisionWorkerPool(ctx context.Context, topology *stormv1beta1.StormTopology, decision *ProvisioningDecision) error {

	if decision.Action == ProvisioningActionNone || decision.Action == ProvisioningActionUse {
		// No action needed
		return nil
	}

	if decision.Action == ProvisioningActionWait {
		return fmt.Errorf("waiting for %s: %s", decision.Reason, decision.Message)
	}

	switch decision.Action {
	case ProvisioningActionCreate:
		return wpp.createWorkerPool(ctx, topology, decision)
	case ProvisioningActionUpdate:
		return wpp.updateWorkerPool(ctx, topology, decision)
	case ProvisioningActionScale:
		return wpp.scaleWorkerPool(ctx, topology, decision)
	default:
		return fmt.Errorf("unsupported provisioning action: %s", decision.Action)
	}
}

// createWorkerPool creates a new worker pool for the topology
func (wpp *WorkerPoolProvisioner) createWorkerPool(ctx context.Context, topology *stormv1beta1.StormTopology, decision *ProvisioningDecision) error {

	if decision.RecommendedSpec == nil {
		return fmt.Errorf("no worker pool specification provided for creation")
	}

	// Create the worker pool resource
	workerPool := &stormv1beta1.StormWorkerPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      decision.WorkerPoolName,
			Namespace: topology.Namespace,
			Labels: map[string]string{
				"storm.apache.org/cluster":        topology.Spec.ClusterRef,
				"storm.apache.org/topology":       topology.Name,
				"storm.apache.org/provisioned-by": "coordinator",
			},
		},
		Spec: *decision.RecommendedSpec,
	}

	// Set topology as owner reference for garbage collection
	if err := controllerutil.SetControllerReference(topology, workerPool, wpp.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Create the worker pool
	if err := wpp.Create(ctx, workerPool); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create worker pool: %w", err)
	}

	return nil
}

// updateWorkerPool updates an existing worker pool
func (wpp *WorkerPoolProvisioner) updateWorkerPool(ctx context.Context, topology *stormv1beta1.StormTopology, decision *ProvisioningDecision) error {
	// Get existing worker pool
	existingPool := &stormv1beta1.StormWorkerPool{}
	if err := wpp.Get(ctx, types.NamespacedName{
		Name:      decision.WorkerPoolName,
		Namespace: topology.Namespace,
	}, existingPool); err != nil {
		return fmt.Errorf("failed to get worker pool %s: %w", decision.WorkerPoolName, err)
	}

	// Update the spec with recommended changes
	if decision.RecommendedSpec != nil {
		existingPool.Spec = *decision.RecommendedSpec
	}

	// Update the worker pool
	if err := wpp.Update(ctx, existingPool); err != nil {
		return fmt.Errorf("failed to update worker pool: %w", err)
	}

	return nil
}

// scaleWorkerPool scales an existing worker pool
func (wpp *WorkerPoolProvisioner) scaleWorkerPool(ctx context.Context, topology *stormv1beta1.StormTopology, decision *ProvisioningDecision) error {
	// Get existing worker pool
	existingPool := &stormv1beta1.StormWorkerPool{}
	if err := wpp.Get(ctx, types.NamespacedName{
		Name:      decision.WorkerPoolName,
		Namespace: topology.Namespace,
	}, existingPool); err != nil {
		return fmt.Errorf("failed to get worker pool %s: %w", decision.WorkerPoolName, err)
	}

	// Update replicas if specified
	if decision.RecommendedSpec != nil && decision.RecommendedSpec.Replicas > 0 {
		existingPool.Spec.Replicas = decision.RecommendedSpec.Replicas
	}

	// Update the worker pool
	if err := wpp.Update(ctx, existingPool); err != nil {
		return fmt.Errorf("failed to scale worker pool: %w", err)
	}

	return nil
}

// findExistingWorkerPools finds worker pools related to the topology
func (wpp *WorkerPoolProvisioner) findExistingWorkerPools(ctx context.Context, topology *stormv1beta1.StormTopology) ([]stormv1beta1.StormWorkerPool, error) {
	pools := &stormv1beta1.StormWorkerPoolList{}

	// List worker pools in the same namespace
	if err := wpp.List(ctx, pools, client.InNamespace(topology.Namespace)); err != nil {
		return nil, err
	}

	var relevantPools []stormv1beta1.StormWorkerPool
	for _, pool := range pools.Items {
		// Check if pool is related to this topology or cluster
		if pool.Spec.TopologyRef == topology.Name || pool.Spec.ClusterRef == topology.Spec.ClusterRef {
			relevantPools = append(relevantPools, pool)
		}
	}

	return relevantPools, nil
}

// findSuitablePool finds a suitable existing worker pool for the topology
func (wpp *WorkerPoolProvisioner) findSuitablePool(pools []stormv1beta1.StormWorkerPool, requirements *WorkerPoolRequirement) *stormv1beta1.StormWorkerPool {
	for _, pool := range pools {
		// Check if pool has sufficient capacity
		if pool.Spec.Replicas >= requirements.MinWorkers {
			// Check if pool is in ready state
			if pool.Status.Phase == "Ready" {
				return &pool
			}
		}
	}
	return nil
}

// generateWorkerPoolName generates a unique name for a worker pool
func (wpp *WorkerPoolProvisioner) generateWorkerPoolName(topology *stormv1beta1.StormTopology) string {
	return fmt.Sprintf("%s-workers", topology.Name)
}

// topologyRequiresDedicatedPool determines if a topology requires its own worker pool
func (wpp *WorkerPoolProvisioner) topologyRequiresDedicatedPool(topology *stormv1beta1.StormTopology) bool {
	// Check for special resource requirements
	if topology.Spec.Workers != nil {
		// If topology specifies custom resources, it might need dedicated pool
		if topology.Spec.Workers.Resources.Requests != nil || topology.Spec.Workers.Resources.Limits != nil {
			return true
		}

		// If topology requires specific number of workers
		if topology.Spec.Workers.Replicas > 2 {
			return true
		}
	}

	// Check for topology-specific configuration that might require isolation
	if topology.Spec.Topology.Config != nil {
		// Look for configurations that suggest resource isolation needs
		if _, hasCustomMem := topology.Spec.Topology.Config["worker.heap.memory.mb"]; hasCustomMem {
			return true
		}
		if _, hasCustomCPU := topology.Spec.Topology.Config["topology.worker.max.heap.size.mb"]; hasCustomCPU {
			return true
		}
	}

	return false
}

// buildWorkerPoolSpec builds a worker pool specification based on topology requirements
func (wpp *WorkerPoolProvisioner) buildWorkerPoolSpec(topology *stormv1beta1.StormTopology, requirements *WorkerPoolRequirement) *stormv1beta1.StormWorkerPoolSpec {
	spec := &stormv1beta1.StormWorkerPoolSpec{
		TopologyRef: topology.Name,
		ClusterRef:  topology.Spec.ClusterRef,
		Replicas:    requirements.MinWorkers,
	}

	// Add resource requirements if specified
	if requirements.RecommendedCPU != "" || requirements.RecommendedRAM != "" {
		resources := corev1.ResourceRequirements{
			Requests: make(corev1.ResourceList),
			Limits:   make(corev1.ResourceList),
		}

		// Parse and set CPU requirements
		if requirements.RecommendedCPU != "" {
			if cpu, err := resource.ParseQuantity(requirements.RecommendedCPU); err == nil {
				resources.Requests[corev1.ResourceCPU] = cpu
				// Set limit to 2x request for burstable workloads
				cpuLimit := cpu.DeepCopy()
				cpuLimit.Add(cpu) // Double the CPU for limits
				resources.Limits[corev1.ResourceCPU] = cpuLimit
			}
		}

		// Parse and set memory requirements
		if requirements.RecommendedRAM != "" {
			if memory, err := resource.ParseQuantity(requirements.RecommendedRAM); err == nil {
				resources.Requests[corev1.ResourceMemory] = memory
				resources.Limits[corev1.ResourceMemory] = memory
			}
		}

		spec.Template = &stormv1beta1.PodTemplateSpec{
			Spec: &stormv1beta1.PodSpecOverride{
				Containers: []stormv1beta1.ContainerOverride{
					{
						Name:      "worker",
						Resources: resources,
					},
				},
			},
		}
	}

	// Add labels from requirements
	if len(requirements.RequiredLabels) > 0 {
		if spec.Template == nil {
			spec.Template = &stormv1beta1.PodTemplateSpec{}
		}
		if spec.Template.Metadata == nil {
			spec.Template.Metadata = &stormv1beta1.PodMetadata{}
		}
		if spec.Template.Metadata.Labels == nil {
			spec.Template.Metadata.Labels = make(map[string]string)
		}

		for key, value := range requirements.RequiredLabels {
			spec.Template.Metadata.Labels[key] = value
		}
	}

	return spec
}

// calculateResourceCost estimates the cost of the provisioning decision
func (wpp *WorkerPoolProvisioner) calculateResourceCost(requirements *WorkerPoolRequirement) *ResourceCost {
	cost := &ResourceCost{}

	// Parse CPU requirements
	if requirements.RecommendedCPU != "" {
		if cpu, err := resource.ParseQuantity(requirements.RecommendedCPU); err == nil {
			cost.CPUCores = float64(cpu.MilliValue()) / 1000.0 * float64(requirements.MinWorkers)
		}
	}

	// Parse memory requirements
	if requirements.RecommendedRAM != "" {
		if memory, err := resource.ParseQuantity(requirements.RecommendedRAM); err == nil {
			cost.MemoryGB = float64(memory.Value()) / (1024 * 1024 * 1024) * float64(requirements.MinWorkers)
		}
	}

	// Estimate storage (assume 10GB per worker)
	cost.StorageGB = 10.0 * float64(requirements.MinWorkers)

	// Simple cost estimation (this would typically integrate with cloud pricing APIs)
	cost.EstimatedCost = fmt.Sprintf("$%.2f/hour", (cost.CPUCores*0.05)+(cost.MemoryGB*0.01)+(cost.StorageGB*0.001))

	return cost
}
