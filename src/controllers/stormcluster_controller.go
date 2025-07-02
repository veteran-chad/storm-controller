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

package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/apache/storm/storm-controller/api/v1beta1"
	"github.com/apache/storm/storm-controller/pkg/metrics"
	"github.com/apache/storm/storm-controller/pkg/storm"
)

// StormClusterReconciler reconciles a StormCluster object
type StormClusterReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	StormClient storm.Client
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/finalizers,verbs=update

// Reconcile handles StormCluster reconciliation
func (r *StormClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormCluster instance
	cluster := &stormv1beta1.StormCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get cluster information from Storm
	clusterInfo, err := r.StormClient.GetClusterInfo(ctx)
	if err != nil {
		log.Error(err, "Failed to get cluster info from Storm")
		cluster.Status.Phase = "Failed"
		cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}
		
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             "ConnectionFailed",
			Message:            err.Error(),
		})
		
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		
		// Retry after 30 seconds
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Update cluster status
	cluster.Status.Phase = "Running"
	cluster.Status.NimbusLeader = clusterInfo.NimbusLeader
	cluster.Status.NimbusNodes = clusterInfo.NimbusHosts
	cluster.Status.SupervisorReady = int32(clusterInfo.Supervisors)
	cluster.Status.TotalSlots = int32(clusterInfo.TotalSlots)
	cluster.Status.UsedSlots = int32(clusterInfo.UsedSlots)
	cluster.Status.FreeSlots = int32(clusterInfo.FreeSlots)
	cluster.Status.TopologyCount = int32(clusterInfo.Topologies)
	cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Update metrics
	labels := map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
	}
	
	// Cluster info metric (version would come from cluster spec or detected version)
	metrics.StormClusterInfo.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"version":   "2.6.0", // TODO: Get from cluster
	}).Set(1)
	
	metrics.StormClusterSupervisors.With(labels).Set(float64(clusterInfo.Supervisors))
	
	// Slot metrics by state
	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "total",
	}).Set(float64(clusterInfo.TotalSlots))
	
	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "used",
	}).Set(float64(clusterInfo.UsedSlots))
	
	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "free",
	}).Set(float64(clusterInfo.FreeSlots))

	// Set conditions
	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
		Reason:             "ClusterHealthy",
		Message:            "Storm cluster is healthy and available",
	})

	// Check if cluster is degraded
	if cluster.Status.FreeSlots == 0 {
		cluster.Status.Phase = "Running"
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "ResourcesAvailable",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             "NoFreeSlots",
			Message:            "No free worker slots available",
		})
	}

	if err := r.Status().Update(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to update status periodically
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StormClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormCluster{}).
		Complete(r)
}