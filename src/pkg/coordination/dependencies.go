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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

// DependencyType represents the type of dependency relationship
type DependencyType string

const (
	// DependencyTypeRequired means the dependency must be satisfied before proceeding
	DependencyTypeRequired DependencyType = "Required"
	// DependencyTypeOptional means the dependency is preferred but not required
	DependencyTypeOptional DependencyType = "Optional"
	// DependencyTypeBlocking means the dependency prevents the operation
	DependencyTypeBlocking DependencyType = "Blocking"
)

// DependencyStatus represents the status of a dependency
type DependencyStatus string

const (
	// DependencyStatusSatisfied means the dependency is satisfied
	DependencyStatusSatisfied DependencyStatus = "Satisfied"
	// DependencyStatusPending means the dependency is not yet satisfied
	DependencyStatusPending DependencyStatus = "Pending"
	// DependencyStatusFailed means the dependency failed
	DependencyStatusFailed DependencyStatus = "Failed"
	// DependencyStatusBlocked means the dependency is blocking the operation
	DependencyStatusBlocked DependencyStatus = "Blocked"
)

// Dependency represents a dependency relationship between resources
type Dependency struct {
	Type          DependencyType
	ResourceType  string
	ResourceName  string
	ResourceUID   types.UID
	Namespace     string
	RequiredPhase string
	Description   string
	CheckInterval time.Duration
	MaxWaitTime   time.Duration
}

// DependencyResult represents the result of a dependency check
type DependencyResult struct {
	Dependency      *Dependency
	Status          DependencyStatus
	CurrentPhase    string
	Message         string
	RecommendedWait time.Duration
	ShouldRequeue   bool
}

// DependencyManager manages dependency relationships between Storm resources
type DependencyManager struct {
	client.Client
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager(client client.Client) *DependencyManager {
	return &DependencyManager{
		Client: client,
	}
}

// CheckDependency checks if a single dependency is satisfied
func (dm *DependencyManager) CheckDependency(ctx context.Context, dep *Dependency) (*DependencyResult, error) {
	log := log.FromContext(ctx)

	result := &DependencyResult{
		Dependency:      dep,
		RecommendedWait: dep.CheckInterval,
		ShouldRequeue:   true,
	}

	switch dep.ResourceType {
	case "StormCluster":
		return dm.checkClusterDependency(ctx, dep, result)
	case "StormTopology":
		return dm.checkTopologyDependency(ctx, dep, result)
	case "StormWorkerPool":
		return dm.checkWorkerPoolDependency(ctx, dep, result)
	default:
		log.Error(fmt.Errorf("unknown resource type"), "Unknown dependency resource type", "type", dep.ResourceType)
		result.Status = DependencyStatusFailed
		result.Message = fmt.Sprintf("Unknown resource type: %s", dep.ResourceType)
		result.ShouldRequeue = false
		return result, nil
	}
}

// checkClusterDependency checks StormCluster dependency
func (dm *DependencyManager) checkClusterDependency(ctx context.Context, dep *Dependency, result *DependencyResult) (*DependencyResult, error) {
	cluster := &stormv1beta1.StormCluster{}
	if err := dm.Get(ctx, types.NamespacedName{
		Name:      dep.ResourceName,
		Namespace: dep.Namespace,
	}, cluster); err != nil {
		result.Status = DependencyStatusFailed
		result.Message = fmt.Sprintf("Failed to get StormCluster %s: %v", dep.ResourceName, err)
		return result, nil
	}

	result.CurrentPhase = cluster.Status.Phase

	// Check if cluster has the required phase
	if cluster.Status.Phase == dep.RequiredPhase {
		result.Status = DependencyStatusSatisfied
		result.Message = fmt.Sprintf("StormCluster %s is in required phase %s", dep.ResourceName, dep.RequiredPhase)
		result.ShouldRequeue = false
		return result, nil
	}

	// Check if cluster is in a terminal failure state
	if cluster.Status.Phase == "Failed" && dep.RequiredPhase != "Failed" {
		if dep.Type == DependencyTypeRequired {
			result.Status = DependencyStatusBlocked
			result.Message = fmt.Sprintf("StormCluster %s is in Failed state", dep.ResourceName)
			result.RecommendedWait = 5 * time.Minute // Wait longer for failed clusters
		} else {
			result.Status = DependencyStatusFailed
			result.Message = fmt.Sprintf("StormCluster %s is in Failed state", dep.ResourceName)
		}
		return result, nil
	}

	// Dependency is pending
	result.Status = DependencyStatusPending
	result.Message = fmt.Sprintf("StormCluster %s is in %s phase, waiting for %s",
		dep.ResourceName, cluster.Status.Phase, dep.RequiredPhase)

	return result, nil
}

// checkTopologyDependency checks StormTopology dependency
func (dm *DependencyManager) checkTopologyDependency(ctx context.Context, dep *Dependency, result *DependencyResult) (*DependencyResult, error) {
	topology := &stormv1beta1.StormTopology{}
	if err := dm.Get(ctx, types.NamespacedName{
		Name:      dep.ResourceName,
		Namespace: dep.Namespace,
	}, topology); err != nil {
		result.Status = DependencyStatusFailed
		result.Message = fmt.Sprintf("Failed to get StormTopology %s: %v", dep.ResourceName, err)
		return result, nil
	}

	result.CurrentPhase = topology.Status.Phase

	// Check if topology has the required phase
	if topology.Status.Phase == dep.RequiredPhase {
		result.Status = DependencyStatusSatisfied
		result.Message = fmt.Sprintf("StormTopology %s is in required phase %s", dep.ResourceName, dep.RequiredPhase)
		result.ShouldRequeue = false
		return result, nil
	}

	// Check for blocking states
	if (topology.Status.Phase == "Failed" || topology.Status.Phase == "Killed") &&
		dep.RequiredPhase != "Failed" && dep.RequiredPhase != "Killed" {
		result.Status = DependencyStatusBlocked
		result.Message = fmt.Sprintf("StormTopology %s is in terminal state %s", dep.ResourceName, topology.Status.Phase)
		return result, nil
	}

	// Dependency is pending
	result.Status = DependencyStatusPending
	result.Message = fmt.Sprintf("StormTopology %s is in %s phase, waiting for %s",
		dep.ResourceName, topology.Status.Phase, dep.RequiredPhase)

	return result, nil
}

// checkWorkerPoolDependency checks StormWorkerPool dependency
func (dm *DependencyManager) checkWorkerPoolDependency(ctx context.Context, dep *Dependency, result *DependencyResult) (*DependencyResult, error) {
	workerPool := &stormv1beta1.StormWorkerPool{}
	if err := dm.Get(ctx, types.NamespacedName{
		Name:      dep.ResourceName,
		Namespace: dep.Namespace,
	}, workerPool); err != nil {
		result.Status = DependencyStatusFailed
		result.Message = fmt.Sprintf("Failed to get StormWorkerPool %s: %v", dep.ResourceName, err)
		return result, nil
	}

	result.CurrentPhase = workerPool.Status.Phase

	// Check if worker pool has the required phase
	if workerPool.Status.Phase == dep.RequiredPhase {
		result.Status = DependencyStatusSatisfied
		result.Message = fmt.Sprintf("StormWorkerPool %s is in required phase %s", dep.ResourceName, dep.RequiredPhase)
		result.ShouldRequeue = false
		return result, nil
	}

	// Check for blocking states
	if workerPool.Status.Phase == "Failed" && dep.RequiredPhase != "Failed" {
		result.Status = DependencyStatusBlocked
		result.Message = fmt.Sprintf("StormWorkerPool %s is in Failed state", dep.ResourceName)
		return result, nil
	}

	// Dependency is pending
	result.Status = DependencyStatusPending
	result.Message = fmt.Sprintf("StormWorkerPool %s is in %s phase, waiting for %s",
		dep.ResourceName, workerPool.Status.Phase, dep.RequiredPhase)

	return result, nil
}

// CheckAllDependencies checks multiple dependencies and returns the overall result
func (dm *DependencyManager) CheckAllDependencies(ctx context.Context, dependencies []*Dependency) ([]*DependencyResult, bool, time.Duration) {
	log := log.FromContext(ctx)

	results := make([]*DependencyResult, len(dependencies))
	allSatisfied := true
	var maxWait time.Duration

	for i, dep := range dependencies {
		result, err := dm.CheckDependency(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to check dependency", "dependency", dep.Description)
			result = &DependencyResult{
				Dependency:      dep,
				Status:          DependencyStatusFailed,
				Message:         fmt.Sprintf("Failed to check dependency: %v", err),
				RecommendedWait: 30 * time.Second,
				ShouldRequeue:   true,
			}
		}

		results[i] = result

		// Update overall status
		switch result.Status {
		case DependencyStatusSatisfied:
			// Continue checking
		case DependencyStatusPending:
			if dep.Type == DependencyTypeRequired {
				allSatisfied = false
				if result.RecommendedWait > maxWait {
					maxWait = result.RecommendedWait
				}
			}
		case DependencyStatusFailed:
			if dep.Type == DependencyTypeRequired {
				allSatisfied = false
				if result.RecommendedWait > maxWait {
					maxWait = result.RecommendedWait
				}
			}
		case DependencyStatusBlocked:
			if dep.Type == DependencyTypeRequired || dep.Type == DependencyTypeBlocking {
				allSatisfied = false
				maxWait = 5 * time.Minute // Longer wait for blocked dependencies
			}
		}
	}

	if maxWait == 0 && !allSatisfied {
		maxWait = 30 * time.Second // Default wait time
	}

	return results, allSatisfied, maxWait
}

// CreateTopologyDependencies creates standard dependencies for a topology
func CreateTopologyDependencies(topology *stormv1beta1.StormTopology) []*Dependency {
	dependencies := make([]*Dependency, 0)

	// Required cluster dependency
	clusterDep := &Dependency{
		Type:          DependencyTypeRequired,
		ResourceType:  "StormCluster",
		ResourceName:  topology.Spec.ClusterRef,
		Namespace:     topology.Namespace,
		RequiredPhase: "Running",
		Description:   fmt.Sprintf("StormCluster %s must be running", topology.Spec.ClusterRef),
		CheckInterval: 30 * time.Second,
		MaxWaitTime:   10 * time.Minute,
	}
	dependencies = append(dependencies, clusterDep)

	// TODO: Add worker pool dependency when WorkerPoolRef field is added to topology spec
	// For now, worker pools are managed separately

	return dependencies
}

// CreateWorkerPoolDependencies creates standard dependencies for a worker pool
func CreateWorkerPoolDependencies(workerPool *stormv1beta1.StormWorkerPool) []*Dependency {
	dependencies := make([]*Dependency, 0)

	// Required cluster dependency
	clusterDep := &Dependency{
		Type:          DependencyTypeRequired,
		ResourceType:  "StormCluster",
		ResourceName:  workerPool.Spec.ClusterRef,
		Namespace:     workerPool.Namespace,
		RequiredPhase: "Running",
		Description:   fmt.Sprintf("StormCluster %s must be running", workerPool.Spec.ClusterRef),
		CheckInterval: 30 * time.Second,
		MaxWaitTime:   10 * time.Minute,
	}
	dependencies = append(dependencies, clusterDep)

	return dependencies
}
