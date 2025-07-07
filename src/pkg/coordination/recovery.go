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

// AutoRecoveryEngine handles automated recovery actions for Storm resources
type AutoRecoveryEngine struct {
	client.Client
	CrossMonitor *CrossResourceMonitor
	Config       *RecoveryConfig
}

// RecoveryConfig configures the recovery engine behavior
type RecoveryConfig struct {
	Enabled                bool
	MaxRecoveryAttempts    int
	RecoveryBackoffBase    time.Duration
	RecoveryBackoffMax     time.Duration
	AutoRestartEnabled     bool
	AutoScaleEnabled       bool
	AutoRepairEnabled      bool
	CriticalIssueThreshold int
	RecoveryWindowDuration time.Duration
}

// NewAutoRecoveryEngine creates a new auto recovery engine
func NewAutoRecoveryEngine(client client.Client, crossMonitor *CrossResourceMonitor) *AutoRecoveryEngine {
	return &AutoRecoveryEngine{
		Client:       client,
		CrossMonitor: crossMonitor,
		Config: &RecoveryConfig{
			Enabled:                true,
			MaxRecoveryAttempts:    3,
			RecoveryBackoffBase:    30 * time.Second,
			RecoveryBackoffMax:     10 * time.Minute,
			AutoRestartEnabled:     true,
			AutoScaleEnabled:       true,
			AutoRepairEnabled:      true,
			CriticalIssueThreshold: 5,
			RecoveryWindowDuration: 1 * time.Hour,
		},
	}
}

// RecoveryPlan represents a plan for recovering from health issues
type RecoveryPlan struct {
	SystemHealth     *SystemHealthStatus
	RecoveryActions  []PlannedRecoveryAction
	EstimatedTime    time.Duration
	RiskLevel        RecoveryRisk
	RequiresApproval bool
	Priority         RecoveryPriority
}

// PlannedRecoveryAction represents a planned recovery action
type PlannedRecoveryAction struct {
	Action        RecoveryAction
	Order         int
	Dependencies  []string
	RiskLevel     RecoveryRisk
	EstimatedTime time.Duration
}

// RecoveryRisk represents the risk level of a recovery action
type RecoveryRisk string

const (
	RiskLevelLow      RecoveryRisk = "Low"
	RiskLevelMedium   RecoveryRisk = "Medium"
	RiskLevelHigh     RecoveryRisk = "High"
	RiskLevelCritical RecoveryRisk = "Critical"
)

// RecoveryPriority represents the priority of recovery actions
type RecoveryPriority string

const (
	PriorityLow      RecoveryPriority = "Low"
	PriorityMedium   RecoveryPriority = "Medium"
	PriorityHigh     RecoveryPriority = "High"
	PriorityCritical RecoveryPriority = "Critical"
)

// AnalyzeAndRecover analyzes system health and executes automated recovery actions
func (are *AutoRecoveryEngine) AnalyzeAndRecover(ctx context.Context, namespace string) (*RecoveryPlan, error) {
	log := log.FromContext(ctx)

	if !are.Config.Enabled {
		log.Info("Auto recovery is disabled")
		return nil, nil
	}

	// Get current system health
	systemHealth, err := are.CrossMonitor.CheckSystemHealth(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check system health: %w", err)
	}

	// Create recovery plan
	plan := &RecoveryPlan{
		SystemHealth:    systemHealth,
		RecoveryActions: make([]PlannedRecoveryAction, 0),
	}

	// Analyze issues and plan recovery actions
	if err := are.planRecoveryActions(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to plan recovery actions: %w", err)
	}

	// Execute recovery actions if appropriate
	if len(plan.RecoveryActions) > 0 {
		log.Info("Executing recovery plan",
			"actions", len(plan.RecoveryActions),
			"priority", plan.Priority,
			"riskLevel", plan.RiskLevel)

		if err := are.executeRecoveryPlan(ctx, plan); err != nil {
			log.Error(err, "Failed to execute recovery plan")
			return plan, err
		}
	}

	return plan, nil
}

// planRecoveryActions analyzes health issues and creates recovery actions
func (are *AutoRecoveryEngine) planRecoveryActions(ctx context.Context, plan *RecoveryPlan) error {
	log := log.FromContext(ctx)

	// Handle critical issues first
	for _, issue := range plan.SystemHealth.CriticalIssues {
		action := are.createRecoveryActionForIssue(issue)
		if action != nil {
			plan.RecoveryActions = append(plan.RecoveryActions, PlannedRecoveryAction{
				Action:        *action,
				Order:         len(plan.RecoveryActions),
				RiskLevel:     are.assessRiskLevel(issue),
				EstimatedTime: are.estimateRecoveryTime(issue),
			})
		}
	}

	// Handle cluster issues
	for _, cluster := range plan.SystemHealth.Clusters {
		if cluster.OverallStatus != HealthStatusHealthy {
			actions := are.createClusterRecoveryActions(cluster)
			for _, action := range actions {
				plan.RecoveryActions = append(plan.RecoveryActions, PlannedRecoveryAction{
					Action:        action,
					Order:         len(plan.RecoveryActions),
					RiskLevel:     RiskLevelMedium,
					EstimatedTime: 2 * time.Minute,
				})
			}
		}
	}

	// Handle topology issues
	for _, topology := range plan.SystemHealth.Topologies {
		if topology.Status != HealthStatusHealthy && len(topology.Issues) > 0 {
			actions := are.createTopologyRecoveryActions(topology)
			for _, action := range actions {
				plan.RecoveryActions = append(plan.RecoveryActions, PlannedRecoveryAction{
					Action:        action,
					Order:         len(plan.RecoveryActions),
					RiskLevel:     RiskLevelLow,
					EstimatedTime: 1 * time.Minute,
				})
			}
		}
	}

	// Handle worker pool issues
	for _, pool := range plan.SystemHealth.WorkerPools {
		if pool.Status != HealthStatusHealthy && len(pool.Issues) > 0 {
			actions := are.createWorkerPoolRecoveryActions(pool)
			for _, action := range actions {
				plan.RecoveryActions = append(plan.RecoveryActions, PlannedRecoveryAction{
					Action:        action,
					Order:         len(plan.RecoveryActions),
					RiskLevel:     RiskLevelLow,
					EstimatedTime: 30 * time.Second,
				})
			}
		}
	}

	// Assess overall plan risk and priority
	plan.RiskLevel = are.assessPlanRisk(plan)
	plan.Priority = are.assessPlanPriority(plan)
	plan.RequiresApproval = are.requiresApproval(plan)

	log.Info("Recovery plan created",
		"actions", len(plan.RecoveryActions),
		"riskLevel", plan.RiskLevel,
		"priority", plan.Priority,
		"requiresApproval", plan.RequiresApproval)

	return nil
}

// createRecoveryActionForIssue creates a recovery action for a specific health issue
func (are *AutoRecoveryEngine) createRecoveryActionForIssue(issue HealthIssue) *RecoveryAction {
	if !issue.AutoRecoverable {
		return nil
	}

	switch issue.Type {
	case IssueTypeResource:
		return &RecoveryAction{
			Type:        RecoveryActionRestart,
			Target:      issue.Component,
			Description: fmt.Sprintf("Restart %s to resolve resource issue", issue.Component),
			ExecutedAt:  time.Now(),
		}
	case IssueTypeCapacity:
		return &RecoveryAction{
			Type:        RecoveryActionScale,
			Target:      issue.Component,
			Description: fmt.Sprintf("Scale %s to resolve capacity issue", issue.Component),
			ExecutedAt:  time.Now(),
		}
	case IssueTypePerformance:
		return &RecoveryAction{
			Type:        RecoveryActionRepair,
			Target:      issue.Component,
			Description: fmt.Sprintf("Repair %s to resolve performance issue", issue.Component),
			ExecutedAt:  time.Now(),
		}
	default:
		return &RecoveryAction{
			Type:        RecoveryActionAlert,
			Target:      issue.Component,
			Description: fmt.Sprintf("Alert for %s issue: %s", issue.Type, issue.Message),
			ExecutedAt:  time.Now(),
		}
	}
}

// createClusterRecoveryActions creates recovery actions for cluster issues
func (are *AutoRecoveryEngine) createClusterRecoveryActions(cluster ClusterHealth) []RecoveryAction {
	actions := make([]RecoveryAction, 0)

	// Check for component failures
	for _, component := range cluster.Components {
		if component.Status == HealthStatusUnhealthy {
			actions = append(actions, RecoveryAction{
				Type:        RecoveryActionRestart,
				Target:      fmt.Sprintf("cluster-%s", component.Component),
				Description: fmt.Sprintf("Restart unhealthy cluster component: %s", component.Component),
				ExecutedAt:  time.Now(),
			})
		}
	}

	return actions
}

// createTopologyRecoveryActions creates recovery actions for topology issues
func (are *AutoRecoveryEngine) createTopologyRecoveryActions(topology TopologyHealth) []RecoveryAction {
	actions := make([]RecoveryAction, 0)

	// Handle failed topologies
	if topology.Status == HealthStatusUnhealthy && topology.Phase == "Failed" {
		actions = append(actions, RecoveryAction{
			Type:        RecoveryActionRestart,
			Target:      topology.Name,
			Namespace:   topology.Namespace,
			Description: fmt.Sprintf("Restart failed topology: %s", topology.Name),
			ExecutedAt:  time.Now(),
		})
	}

	// Handle dependency issues
	for _, issue := range topology.Issues {
		if issue.Type == IssueTypeDependency && issue.AutoRecoverable {
			actions = append(actions, RecoveryAction{
				Type:        RecoveryActionRepair,
				Target:      topology.Name,
				Namespace:   topology.Namespace,
				Description: fmt.Sprintf("Repair topology dependency: %s", issue.Message),
				ExecutedAt:  time.Now(),
			})
		}
	}

	return actions
}

// createWorkerPoolRecoveryActions creates recovery actions for worker pool issues
func (are *AutoRecoveryEngine) createWorkerPoolRecoveryActions(pool WorkerPoolHealth) []RecoveryAction {
	actions := make([]RecoveryAction, 0)

	// Handle capacity issues
	for _, issue := range pool.Issues {
		if issue.Type == IssueTypeCapacity && issue.AutoRecoverable {
			actions = append(actions, RecoveryAction{
				Type:        RecoveryActionScale,
				Target:      pool.Name,
				Namespace:   pool.Namespace,
				Description: fmt.Sprintf("Scale worker pool to resolve capacity issue: %s", pool.Name),
				ExecutedAt:  time.Now(),
			})
		}
	}

	return actions
}

// executeRecoveryPlan executes the recovery plan
func (are *AutoRecoveryEngine) executeRecoveryPlan(ctx context.Context, plan *RecoveryPlan) error {
	log := log.FromContext(ctx)

	if plan.RequiresApproval {
		log.Info("Recovery plan requires approval, skipping automatic execution")
		return nil
	}

	// Execute actions in order
	for i, plannedAction := range plan.RecoveryActions {
		log.Info("Executing recovery action",
			"order", i+1,
			"type", plannedAction.Action.Type,
			"target", plannedAction.Action.Target)

		if err := are.executeRecoveryAction(ctx, &plannedAction.Action); err != nil {
			log.Error(err, "Recovery action failed", "action", plannedAction.Action.Type)
			plannedAction.Action.Success = false
			plannedAction.Action.ErrorMessage = err.Error()
		} else {
			plannedAction.Action.Success = true
			log.Info("Recovery action completed successfully", "action", plannedAction.Action.Type)
		}

		// Wait between actions to allow system to stabilize
		if i < len(plan.RecoveryActions)-1 {
			time.Sleep(are.Config.RecoveryBackoffBase)
		}
	}

	return nil
}

// executeRecoveryAction executes a specific recovery action
func (are *AutoRecoveryEngine) executeRecoveryAction(ctx context.Context, action *RecoveryAction) error {
	switch action.Type {
	case RecoveryActionRestart:
		return are.executeRestartAction(ctx, action)
	case RecoveryActionScale:
		return are.executeScaleAction(ctx, action)
	case RecoveryActionRepair:
		return are.executeRepairAction(ctx, action)
	case RecoveryActionReschedule:
		return are.executeRescheduleAction(ctx, action)
	case RecoveryActionAlert:
		return are.executeAlertAction(ctx, action)
	default:
		return fmt.Errorf("unknown recovery action type: %s", action.Type)
	}
}

// executeRestartAction restarts a Storm resource
func (are *AutoRecoveryEngine) executeRestartAction(ctx context.Context, action *RecoveryAction) error {
	log := log.FromContext(ctx)

	// Handle topology restart
	if action.Target != "" && action.Namespace != "" {
		topology := &stormv1beta1.StormTopology{}
		if err := are.Get(ctx, types.NamespacedName{
			Name:      action.Target,
			Namespace: action.Namespace,
		}, topology); err == nil {
			// Trigger topology restart by updating an annotation
			if topology.Annotations == nil {
				topology.Annotations = make(map[string]string)
			}
			topology.Annotations["storm.apache.org/restart-requested"] = time.Now().Format(time.RFC3339)

			if err := are.Update(ctx, topology); err != nil {
				return fmt.Errorf("failed to trigger topology restart: %w", err)
			}

			log.Info("Triggered topology restart", "topology", action.Target)
			return nil
		}
	}

	// Handle other restart actions (clusters, worker pools)
	log.Info("Restart action logged", "target", action.Target)
	return nil
}

// executeScaleAction scales a worker pool
func (are *AutoRecoveryEngine) executeScaleAction(ctx context.Context, action *RecoveryAction) error {
	if action.Target == "" || action.Namespace == "" {
		return fmt.Errorf("scale action requires target and namespace")
	}

	workerPool := &stormv1beta1.StormWorkerPool{}
	if err := are.Get(ctx, types.NamespacedName{
		Name:      action.Target,
		Namespace: action.Namespace,
	}, workerPool); err != nil {
		return fmt.Errorf("failed to get worker pool %s: %w", action.Target, err)
	}

	// Increase replicas by 1 (simple scaling strategy)
	originalReplicas := workerPool.Spec.Replicas
	workerPool.Spec.Replicas = originalReplicas + 1

	if err := are.Update(ctx, workerPool); err != nil {
		return fmt.Errorf("failed to scale worker pool: %w", err)
	}

	log.FromContext(ctx).Info("Scaled worker pool",
		"pool", action.Target,
		"originalReplicas", originalReplicas,
		"newReplicas", workerPool.Spec.Replicas)

	return nil
}

// executeRepairAction performs repair operations
func (are *AutoRecoveryEngine) executeRepairAction(ctx context.Context, action *RecoveryAction) error {
	log := log.FromContext(ctx)

	// For now, repair actions are mainly informational
	// In a full implementation, this could trigger various repair procedures
	log.Info("Repair action executed", "target", action.Target, "description", action.Description)
	return nil
}

// executeRescheduleAction reschedules resources
func (are *AutoRecoveryEngine) executeRescheduleAction(ctx context.Context, action *RecoveryAction) error {
	log := log.FromContext(ctx)

	// Reschedule actions would typically involve pod eviction and rescheduling
	log.Info("Reschedule action executed", "target", action.Target)
	return nil
}

// executeAlertAction sends alerts
func (are *AutoRecoveryEngine) executeAlertAction(ctx context.Context, action *RecoveryAction) error {
	log := log.FromContext(ctx)

	// In a real implementation, this would integrate with alerting systems
	log.Info("ALERT", "target", action.Target, "description", action.Description)
	return nil
}

// Helper methods for assessment
func (are *AutoRecoveryEngine) assessRiskLevel(issue HealthIssue) RecoveryRisk {
	switch issue.Severity {
	case SeverityCritical:
		return RiskLevelCritical
	case SeverityHigh:
		return RiskLevelHigh
	case SeverityMedium:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

func (are *AutoRecoveryEngine) estimateRecoveryTime(issue HealthIssue) time.Duration {
	switch issue.Type {
	case IssueTypeResource:
		return 2 * time.Minute
	case IssueTypeCapacity:
		return 1 * time.Minute
	case IssueTypePerformance:
		return 5 * time.Minute
	default:
		return 30 * time.Second
	}
}

func (are *AutoRecoveryEngine) assessPlanRisk(plan *RecoveryPlan) RecoveryRisk {
	maxRisk := RiskLevelLow
	for _, action := range plan.RecoveryActions {
		if action.RiskLevel > maxRisk {
			maxRisk = action.RiskLevel
		}
	}
	return maxRisk
}

func (are *AutoRecoveryEngine) assessPlanPriority(plan *RecoveryPlan) RecoveryPriority {
	if len(plan.SystemHealth.CriticalIssues) > 0 {
		return PriorityCritical
	}
	if plan.SystemHealth.HealthScore < 50 {
		return PriorityHigh
	}
	if plan.SystemHealth.HealthScore < 80 {
		return PriorityMedium
	}
	return PriorityLow
}

func (are *AutoRecoveryEngine) requiresApproval(plan *RecoveryPlan) bool {
	// Require approval for high-risk or critical priority plans
	return plan.RiskLevel >= RiskLevelHigh || plan.Priority == PriorityCritical
}
