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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/apache/storm/storm-controller/api/v1beta1"
	"github.com/apache/storm/storm-controller/pkg/metrics"
)

// StormWorkerPoolReconciler reconciles a StormWorkerPool object
type StormWorkerPoolReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles StormWorkerPool reconciliation
func (r *StormWorkerPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormWorkerPool instance
	workerPool := &stormv1beta1.StormWorkerPool{}
	if err := r.Get(ctx, req.NamespacedName, workerPool); err != nil {
		// Ignore not found errors
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// For now, just update status to indicate we're managing it
	log.Info("Managing StormWorkerPool", 
		"name", workerPool.Name, 
		"topologyRef", workerPool.Spec.TopologyRef,
		"replicas", workerPool.Spec.Replicas)

	// Update status
	workerPool.Status.Phase = "Running"
	workerPool.Status.ReadyReplicas = workerPool.Spec.Replicas
	workerPool.Status.Replicas = workerPool.Spec.Replicas
	
	// Update metrics
	metrics.StormWorkerPoolReplicas.With(map[string]string{
		"pool":      workerPool.Name,
		"namespace": workerPool.Namespace,
		"topology":  workerPool.Spec.TopologyRef,
		"state":     "desired",
	}).Set(float64(workerPool.Spec.Replicas))
	
	metrics.StormWorkerPoolReplicas.With(map[string]string{
		"pool":      workerPool.Name,
		"namespace": workerPool.Namespace,
		"topology":  workerPool.Spec.TopologyRef,
		"state":     "ready",
	}).Set(float64(workerPool.Status.ReadyReplicas))
	
	if err := r.Status().Update(ctx, workerPool); err != nil {
		log.Error(err, "Failed to update worker pool status")
		return ctrl.Result{}, err
	}
	
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StormWorkerPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormWorkerPool{}).
		Complete(r)
}