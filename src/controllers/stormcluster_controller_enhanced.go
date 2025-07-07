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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
	"github.com/veteran-chad/storm-controller/pkg/metrics"
	"github.com/veteran-chad/storm-controller/pkg/storm"
)

const (
	clusterFinalizer = "storm.apache.org/cluster-finalizer"
	stormConfigName  = "storm-config"
)

// StormClusterReconcilerEnhanced reconciles a StormCluster object with provisioning capability
type StormClusterReconcilerEnhanced struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientManager storm.ClientManager
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies,verbs=get;list;watch

// Reconcile handles StormCluster reconciliation
func (r *StormClusterReconcilerEnhanced) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormCluster instance
	cluster := &stormv1beta1.StormCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if cluster.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, cluster)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		controllerutil.AddFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set initial status if not set
	if cluster.Status.Phase == "" {
		cluster.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile cluster resources
	if err := r.reconcileClusterResources(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile cluster resources")
		return r.updateStatusError(ctx, cluster, err)
	}

	// Check cluster health
	if err := r.checkClusterHealth(ctx, cluster); err != nil {
		log.Error(err, "Failed to check cluster health")
		// Don't fail reconciliation, just update status
		cluster.Status.Phase = "Degraded"
	}

	// Update status
	if err := r.updateStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to check status periodically
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// reconcileClusterResources reconciles all cluster resources
func (r *StormClusterReconcilerEnhanced) reconcileClusterResources(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	// Update phase to Creating if in Pending
	if cluster.Status.Phase == "Pending" {
		cluster.Status.Phase = "Creating"
		if err := r.Status().Update(ctx, cluster); err != nil {
			return err
		}
	}

	// Create ConfigMap for Storm configuration
	if err := r.reconcileConfigMap(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	// Create Zookeeper if enabled (using external for now)
	if cluster.Spec.Zookeeper.Enabled {
		log.Info("Using external Zookeeper configuration", "servers", cluster.Spec.Zookeeper.ExternalServers)
		// In a full implementation, we would create a Zookeeper StatefulSet here
	}

	// Create Nimbus StatefulSet
	if err := r.reconcileNimbus(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Nimbus: %w", err)
	}

	// Create Supervisor Deployment/DaemonSet
	if err := r.reconcileSupervisors(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Supervisors: %w", err)
	}

	// Create UI Deployment if enabled
	if cluster.Spec.UI.Enabled {
		if err := r.reconcileUI(ctx, cluster); err != nil {
			return fmt.Errorf("failed to reconcile UI: %w", err)
		}
	}

	// Create Services
	if err := r.reconcileServices(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile Services: %w", err)
	}

	return nil
}

// reconcileConfigMap creates or updates the Storm configuration ConfigMap
func (r *StormClusterReconcilerEnhanced) reconcileConfigMap(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stormConfigName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, configMap, r.Scheme); err != nil {
			return err
		}

		// Build Storm configuration
		stormConfig := r.buildStormConfig(cluster)
		configMap.Data = map[string]string{
			"storm.yaml": stormConfig,
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled ConfigMap", "name", configMap.Name)
	return nil
}

// buildStormConfig builds the storm.yaml configuration
func (r *StormClusterReconcilerEnhanced) buildStormConfig(cluster *stormv1beta1.StormCluster) string {
	config := "# Storm configuration\n"

	// Zookeeper configuration
	if len(cluster.Spec.Zookeeper.ExternalServers) > 0 {
		config += "storm.zookeeper.servers:\n"
		for _, server := range cluster.Spec.Zookeeper.ExternalServers {
			config += fmt.Sprintf("  - \"%s\"\n", server)
		}
	}
	config += fmt.Sprintf("storm.zookeeper.root: \"%s\"\n", cluster.Spec.Zookeeper.ChrootPath)

	// Nimbus seeds
	config += "nimbus.seeds:\n"
	for i := 0; i < int(cluster.Spec.Nimbus.Replicas); i++ {
		config += fmt.Sprintf("  - \"%s-nimbus-%d.%s-nimbus.%s.svc.cluster.local\"\n",
			cluster.Name, i, cluster.Name, cluster.Namespace)
	}

	// Nimbus Thrift configuration
	config += fmt.Sprintf("nimbus.thrift.port: %d\n", cluster.Spec.Nimbus.Thrift.Port)

	// Supervisor slots
	config += fmt.Sprintf("supervisor.slots.ports:\n")
	for i := 0; i < int(cluster.Spec.Supervisor.WorkerSlots); i++ {
		config += fmt.Sprintf("  - %d\n", 6700+i)
	}

	// UI configuration
	if cluster.Spec.UI.Enabled {
		config += fmt.Sprintf("ui.port: %d\n", cluster.Spec.UI.Service.Port)
	}

	// Custom configuration
	for key, value := range cluster.Spec.Config {
		config += fmt.Sprintf("%s: %s\n", key, value)
	}

	return config
}

// reconcileNimbus creates or updates the Nimbus StatefulSet
func (r *StormClusterReconcilerEnhanced) reconcileNimbus(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-nimbus",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulSet, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, statefulSet, r.Scheme); err != nil {
			return err
		}

		// Build StatefulSet spec
		statefulSet.Spec = r.buildNimbusStatefulSetSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled Nimbus StatefulSet", "name", statefulSet.Name)
	return nil
}

// buildNimbusStatefulSetSpec builds the Nimbus StatefulSet specification
func (r *StormClusterReconcilerEnhanced) buildNimbusStatefulSetSpec(cluster *stormv1beta1.StormCluster) appsv1.StatefulSetSpec {
	replicas := cluster.Spec.Nimbus.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "nimbus",
		"cluster":   cluster.Name,
	}

	// Build container
	container := corev1.Container{
		Name:    "nimbus",
		Image:   r.getStormImage(cluster),
		Command: []string{"storm", "nimbus"},
		Ports: []corev1.ContainerPort{
			{
				Name:          "thrift",
				ContainerPort: cluster.Spec.Nimbus.Thrift.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: cluster.Spec.Nimbus.Resources,
		Env: append([]corev1.EnvVar{
			{
				Name:  "STORM_CONF_DIR",
				Value: "/conf",
			},
		}, cluster.Spec.Nimbus.ExtraEnvVars...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
			{
				Name:      "nimbus-data",
				MountPath: "/data",
			},
		},
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: r.getImagePullSecrets(cluster),
		NodeSelector:     cluster.Spec.Nimbus.NodeSelector,
		Tolerations:      cluster.Spec.Nimbus.Tolerations,
		Affinity:         cluster.Spec.Nimbus.Affinity,
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: stormConfigName,
						},
					},
				},
			},
		},
	}

	// Build volume claim templates if persistence is enabled
	var volumeClaimTemplates []corev1.PersistentVolumeClaim
	if cluster.Spec.Nimbus.Persistence.Enabled {
		pvcSpec := corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				cluster.Spec.Nimbus.Persistence.AccessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(cluster.Spec.Nimbus.Persistence.Size),
				},
			},
		}

		// Only set StorageClassName if it's not empty
		if cluster.Spec.Nimbus.Persistence.StorageClass != "" {
			pvcSpec.StorageClassName = &cluster.Spec.Nimbus.Persistence.StorageClass
		}

		volumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nimbus-data",
				},
				Spec: pvcSpec,
			},
		}
	} else {
		// Add emptyDir volume if persistence is disabled
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "nimbus-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return appsv1.StatefulSetSpec{
		Replicas:    &replicas,
		ServiceName: cluster.Name + "-nimbus",
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: podSpec,
		},
		VolumeClaimTemplates: volumeClaimTemplates,
	}
}

// reconcileSupervisors creates or updates the Supervisor Deployment/DaemonSet
func (r *StormClusterReconcilerEnhanced) reconcileSupervisors(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	if cluster.Spec.Supervisor.DeploymentMode == "daemonset" {
		return r.reconcileSupervisorDaemonSet(ctx, cluster)
	}

	// Default to Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-supervisor",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, deployment, r.Scheme); err != nil {
			return err
		}

		// Build Deployment spec
		deployment.Spec = r.buildSupervisorDeploymentSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled Supervisor Deployment", "name", deployment.Name)
	return nil
}

// buildSupervisorDeploymentSpec builds the Supervisor Deployment specification
func (r *StormClusterReconcilerEnhanced) buildSupervisorDeploymentSpec(cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := cluster.Spec.Supervisor.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "supervisor",
		"cluster":   cluster.Name,
	}

	// Build container ports
	ports := []corev1.ContainerPort{}
	for i := 0; i < int(cluster.Spec.Supervisor.WorkerSlots); i++ {
		ports = append(ports, corev1.ContainerPort{
			Name:          fmt.Sprintf("worker-%d", i),
			ContainerPort: int32(6700 + i),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Build container
	container := corev1.Container{
		Name:      "supervisor",
		Image:     r.getStormImage(cluster),
		Command:   []string{"storm", "supervisor"},
		Ports:     ports,
		Resources: cluster.Spec.Supervisor.Resources,
		Env: append([]corev1.EnvVar{
			{
				Name:  "STORM_CONF_DIR",
				Value: "/conf",
			},
		}, cluster.Spec.Supervisor.ExtraEnvVars...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
			{
				Name:      "storm-data",
				MountPath: "/data",
			},
		},
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: r.getImagePullSecrets(cluster),
		NodeSelector:     cluster.Spec.Supervisor.NodeSelector,
		Tolerations:      cluster.Spec.Supervisor.Tolerations,
		Affinity:         cluster.Spec.Supervisor.Affinity,
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: stormConfigName,
						},
					},
				},
			},
			{
				Name: "storm-data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: podSpec,
		},
	}
}

// reconcileSupervisorDaemonSet creates or updates the Supervisor DaemonSet
func (r *StormClusterReconcilerEnhanced) reconcileSupervisorDaemonSet(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Similar to Deployment but using DaemonSet
	// Implementation omitted for brevity
	return nil
}

// reconcileUI creates or updates the UI Deployment
func (r *StormClusterReconcilerEnhanced) reconcileUI(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-ui",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, deployment, r.Scheme); err != nil {
			return err
		}

		// Build Deployment spec
		deployment.Spec = r.buildUIDeploymentSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled UI Deployment", "name", deployment.Name)
	return nil
}

// buildUIDeploymentSpec builds the UI Deployment specification
func (r *StormClusterReconcilerEnhanced) buildUIDeploymentSpec(cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := cluster.Spec.UI.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "ui",
		"cluster":   cluster.Name,
	}

	// Build container
	container := corev1.Container{
		Name:    "ui",
		Image:   r.getStormImage(cluster),
		Command: []string{"storm", "ui"},
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: cluster.Spec.UI.Service.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: cluster.Spec.UI.Resources,
		Env: []corev1.EnvVar{
			{
				Name:  "STORM_CONF_DIR",
				Value: "/conf",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
		},
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: r.getImagePullSecrets(cluster),
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: stormConfigName,
						},
					},
				},
			},
		},
	}

	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: podSpec,
		},
	}
}

// reconcileServices creates or updates the cluster services
func (r *StormClusterReconcilerEnhanced) reconcileServices(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Create Nimbus headless service
	if err := r.reconcileNimbusService(ctx, cluster); err != nil {
		return err
	}

	// Create UI service if enabled
	if cluster.Spec.UI.Enabled {
		if err := r.reconcileUIService(ctx, cluster); err != nil {
			return err
		}
	}

	return nil
}

// reconcileNimbusService creates or updates the Nimbus service
func (r *StormClusterReconcilerEnhanced) reconcileNimbusService(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-nimbus",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
			return err
		}

		service.Spec = corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone, // Headless service
			Selector: map[string]string{
				"app":       "storm",
				"component": "nimbus",
				"cluster":   cluster.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "thrift",
					Port:       cluster.Spec.Nimbus.Thrift.Port,
					TargetPort: intstr.FromString("thrift"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled Nimbus Service", "name", service.Name)
	return nil
}

// reconcileUIService creates or updates the UI service
func (r *StormClusterReconcilerEnhanced) reconcileUIService(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name + "-ui",
			Namespace:   cluster.Namespace,
			Annotations: cluster.Spec.UI.Service.Annotations,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
			return err
		}

		service.Spec = corev1.ServiceSpec{
			Type: cluster.Spec.UI.Service.Type,
			Selector: map[string]string{
				"app":       "storm",
				"component": "ui",
				"cluster":   cluster.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       cluster.Spec.UI.Service.Port,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}

		// Set additional fields based on service type
		if cluster.Spec.UI.Service.Type == corev1.ServiceTypeNodePort && cluster.Spec.UI.Service.NodePort > 0 {
			service.Spec.Ports[0].NodePort = cluster.Spec.UI.Service.NodePort
		}
		if cluster.Spec.UI.Service.Type == corev1.ServiceTypeLoadBalancer && cluster.Spec.UI.Service.LoadBalancerIP != "" {
			service.Spec.LoadBalancerIP = cluster.Spec.UI.Service.LoadBalancerIP
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled UI Service", "name", service.Name)
	return nil
}

// checkClusterHealth checks the health of the Storm cluster
func (r *StormClusterReconcilerEnhanced) checkClusterHealth(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	// Check if all components are ready
	nimbusReady, err := r.getReadyReplicas(ctx, cluster.Name+"-nimbus", cluster.Namespace, "nimbus")
	if err != nil {
		return err
	}
	cluster.Status.NimbusReady = nimbusReady

	supervisorReady, err := r.getReadyReplicas(ctx, cluster.Name+"-supervisor", cluster.Namespace, "supervisor")
	if err != nil {
		return err
	}
	cluster.Status.SupervisorReady = supervisorReady

	if cluster.Spec.UI.Enabled {
		uiReady, err := r.getReadyReplicas(ctx, cluster.Name+"-ui", cluster.Namespace, "ui")
		if err != nil {
			return err
		}
		cluster.Status.UIReady = uiReady
	}

	// Try to connect to Storm cluster
	if nimbusReady > 0 {
		// Update Storm client configuration when Nimbus is ready
		clientConfig := &storm.ClientConfig{
			Type:       storm.ClientTypeThrift,
			NimbusHost: fmt.Sprintf("%s-nimbus.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
			NimbusPort: int(cluster.Spec.Nimbus.Thrift.Port),
			UIHost:     fmt.Sprintf("%s-ui.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
			UIPort:     int(cluster.Spec.UI.Service.Port),
		}

		if err := r.ClientManager.UpdateClient(clientConfig); err != nil {
			log.FromContext(ctx).Error(err, "Failed to update Storm client")
			// Continue without failing - client will be updated on next reconcile
		} else {
			// Try to get cluster info if client is available
			if client, err := r.ClientManager.GetClient(); err == nil {
				clusterInfo, err := client.GetClusterInfo(ctx)
				if err == nil {
					cluster.Status.NimbusLeader = clusterInfo.NimbusLeader
					cluster.Status.NimbusNodes = clusterInfo.NimbusHosts
					cluster.Status.TotalSlots = int32(clusterInfo.TotalSlots)
					cluster.Status.UsedSlots = int32(clusterInfo.UsedSlots)
					cluster.Status.FreeSlots = int32(clusterInfo.FreeSlots)
					cluster.Status.TopologyCount = int32(clusterInfo.Topologies)

					// Update phase based on readiness
					if nimbusReady >= cluster.Spec.Nimbus.Replicas &&
						supervisorReady >= cluster.Spec.Supervisor.Replicas {
						cluster.Status.Phase = "Running"
					} else {
						cluster.Status.Phase = "Degraded"
					}
				}
			}
		}
	} else {
		// Remove client when Nimbus is not ready
		r.ClientManager.RemoveClient()
	}

	// Update endpoints
	cluster.Status.Endpoints = stormv1beta1.ClusterEndpoints{
		Nimbus:  fmt.Sprintf("%s-nimbus.%s.svc.cluster.local:%d", cluster.Name, cluster.Namespace, cluster.Spec.Nimbus.Thrift.Port),
		UI:      fmt.Sprintf("%s-ui.%s.svc.cluster.local:%d", cluster.Name, cluster.Namespace, cluster.Spec.UI.Service.Port),
		RestAPI: fmt.Sprintf("http://%s-ui.%s.svc.cluster.local:%d/api/v1", cluster.Name, cluster.Namespace, cluster.Spec.UI.Service.Port),
	}

	return nil
}

// getReadyReplicas gets the number of ready replicas for a component
func (r *StormClusterReconcilerEnhanced) getReadyReplicas(ctx context.Context, name, namespace, component string) (int32, error) {
	// Check StatefulSet for Nimbus
	if component == "nimbus" {
		sts := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts); err != nil {
			if errors.IsNotFound(err) {
				return 0, nil
			}
			return 0, err
		}
		return sts.Status.ReadyReplicas, nil
	}

	// Check Deployment for Supervisor and UI
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment); err != nil {
		if errors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	return deployment.Status.ReadyReplicas, nil
}

// updateStatus updates the cluster status
func (r *StormClusterReconcilerEnhanced) updateStatus(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	// Update metrics
	labels := map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
	}

	metrics.StormClusterInfo.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"version":   cluster.Spec.Image.Tag,
	}).Set(1)

	metrics.StormClusterSupervisors.With(labels).Set(float64(cluster.Status.SupervisorReady))

	// Slot metrics
	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "total",
	}).Set(float64(cluster.Status.TotalSlots))

	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "used",
	}).Set(float64(cluster.Status.UsedSlots))

	metrics.StormClusterSlots.With(map[string]string{
		"cluster":   cluster.Name,
		"namespace": cluster.Namespace,
		"state":     "free",
	}).Set(float64(cluster.Status.FreeSlots))

	// Set conditions
	if cluster.Status.Phase == "Running" {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: cluster.Generation,
			Reason:             "ClusterHealthy",
			Message:            "Storm cluster is healthy and available",
		})
	}

	// Check if cluster is degraded
	if cluster.Status.FreeSlots == 0 && cluster.Status.TotalSlots > 0 {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:               "ResourcesAvailable",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: cluster.Generation,
			Reason:             "NoFreeSlots",
			Message:            "No free worker slots available",
		})
	}

	return r.Status().Update(ctx, cluster)
}

// updateStatusError updates the status with an error
func (r *StormClusterReconcilerEnhanced) updateStatusError(ctx context.Context, cluster *stormv1beta1.StormCluster, err error) (ctrl.Result, error) {
	cluster.Status.Phase = "Failed"
	cluster.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cluster.Generation,
		Reason:             "ReconciliationFailed",
		Message:            err.Error(),
	})

	if statusErr := r.Status().Update(ctx, cluster); statusErr != nil {
		return ctrl.Result{}, statusErr
	}

	// Retry after delay
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleDeletion handles cluster deletion
func (r *StormClusterReconcilerEnhanced) handleDeletion(ctx context.Context, cluster *stormv1beta1.StormCluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(cluster, clusterFinalizer) {
		log.Info("Handling cluster deletion", "cluster", cluster.Name)

		// Cleanup any cluster-specific resources if needed
		// Remove Storm client connection
		r.ClientManager.RemoveClient()

		// For now, Kubernetes will handle deletion of owned resources

		// Remove finalizer
		controllerutil.RemoveFinalizer(cluster, clusterFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Helper functions

func (r *StormClusterReconcilerEnhanced) getStormImage(cluster *stormv1beta1.StormCluster) string {
	registry := cluster.Spec.Image.Registry
	if registry == "" {
		registry = "docker.io"
	}
	repository := cluster.Spec.Image.Repository
	if repository == "" {
		repository = "apache/storm"
	}
	tag := cluster.Spec.Image.Tag
	if tag == "" {
		tag = "2.6.0"
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func (r *StormClusterReconcilerEnhanced) getImagePullSecrets(cluster *stormv1beta1.StormCluster) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	for _, secret := range cluster.Spec.Image.PullSecrets {
		secrets = append(secrets, corev1.LocalObjectReference{Name: secret})
	}
	return secrets
}

// findClusterForTopology returns a list of requests for the cluster referenced by this topology
func (r *StormClusterReconcilerEnhanced) findClusterForTopology(ctx context.Context, obj client.Object) []ctrl.Request {
	topology := obj.(*stormv1beta1.StormTopology)

	// Return request for the referenced cluster
	return []ctrl.Request{{
		NamespacedName: types.NamespacedName{
			Name:      topology.Spec.ClusterRef,
			Namespace: topology.Namespace,
		},
	}}
}

// SetupWithManager sets up the controller with the Manager
func (r *StormClusterReconcilerEnhanced) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&stormv1beta1.StormTopology{},
			handler.EnqueueRequestsFromMapFunc(r.findClusterForTopology)).
		Complete(r)
}
