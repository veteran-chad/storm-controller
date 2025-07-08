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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stormv1beta1 "github.com/veteran-chad/storm-controller/api/v1beta1"
)

// ResourceReconciler provides common resource reconciliation methods
type ResourceReconciler interface {
	reconcileNimbus(ctx context.Context, cluster *stormv1beta1.StormCluster) error
	reconcileSupervisors(ctx context.Context, cluster *stormv1beta1.StormCluster) error
	reconcileUI(ctx context.Context, cluster *stormv1beta1.StormCluster) error
	reconcileNimbusService(ctx context.Context, cluster *stormv1beta1.StormCluster) error
	reconcileUIService(ctx context.Context, cluster *stormv1beta1.StormCluster) error
}

// reconcileNimbus creates or updates the Nimbus StatefulSet
func (r *StormClusterReconciler) reconcileNimbus(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	// Determine the StatefulSet name based on management mode
	statefulSetName := cluster.Name + "-nimbus"
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.NimbusStatefulSet != "" {
		statefulSetName = cluster.Spec.ResourceNames.NimbusStatefulSet
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulSet, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, statefulSet, r.Scheme); err != nil {
			return err
		}

		// In reference mode, only update mutable fields
		if cluster.Spec.ManagementMode == "reference" {
			// For existing StatefulSets, we can only update certain fields
			if !statefulSet.CreationTimestamp.IsZero() {
				// Existing StatefulSet - don't modify it in reference mode
				// The controller should only watch/monitor, not update
				return nil
			}
		}

		// In create mode or for new resources, set the full spec
		statefulSet.Spec = buildNimbusStatefulSetSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled Nimbus StatefulSet", "name", statefulSet.Name)
	return nil
}

// reconcileSupervisors creates or updates the Supervisor Deployment
func (r *StormClusterReconciler) reconcileSupervisors(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	// Determine the Deployment name based on management mode
	deploymentName := cluster.Name + "-supervisor"
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.SupervisorDeployment != "" {
		deploymentName = cluster.Spec.ResourceNames.SupervisorDeployment
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, deployment, r.Scheme); err != nil {
			return err
		}

		// In reference mode, don't modify existing deployments
		if cluster.Spec.ManagementMode == "reference" && !deployment.CreationTimestamp.IsZero() {
			return nil
		}

		// Build Deployment spec
		deployment.Spec = buildSupervisorDeploymentSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled Supervisor Deployment", "name", deployment.Name)
	return nil
}

// reconcileUI creates or updates the UI Deployment
func (r *StormClusterReconciler) reconcileUI(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	if !cluster.Spec.UI.Enabled {
		return nil
	}

	log := log.FromContext(ctx)

	// Determine the Deployment name based on management mode
	deploymentName := cluster.Name + "-ui"
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.UIDeployment != "" {
		deploymentName = cluster.Spec.ResourceNames.UIDeployment
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, deployment, r.Scheme); err != nil {
			return err
		}

		// In reference mode, don't modify existing deployments
		if cluster.Spec.ManagementMode == "reference" && !deployment.CreationTimestamp.IsZero() {
			return nil
		}

		// Build Deployment spec
		deployment.Spec = buildUIDeploymentSpec(cluster)

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled UI Deployment", "name", deployment.Name)
	return nil
}

// reconcileNimbusService creates or updates the Nimbus Service
func (r *StormClusterReconciler) reconcileNimbusService(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	log := log.FromContext(ctx)

	// Determine the Service name based on management mode
	serviceName := cluster.Name + "-nimbus"
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.NimbusService != "" {
		serviceName = cluster.Spec.ResourceNames.NimbusService
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
			return err
		}

		// In reference mode, don't modify existing services
		if cluster.Spec.ManagementMode == "reference" && !service.CreationTimestamp.IsZero() {
			return nil
		}

		service.Spec = corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone, // Headless service for StatefulSet
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

// reconcileUIService creates or updates the UI Service
func (r *StormClusterReconciler) reconcileUIService(ctx context.Context, cluster *stormv1beta1.StormCluster) error {
	if !cluster.Spec.UI.Enabled {
		return nil
	}

	log := log.FromContext(ctx)

	// Determine the Service name based on management mode
	serviceName := cluster.Name + "-ui"
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.UIService != "" {
		serviceName = cluster.Spec.ResourceNames.UIService
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
			return err
		}

		// In reference mode, don't modify existing services
		if cluster.Spec.ManagementMode == "reference" && !service.CreationTimestamp.IsZero() {
			return nil
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

// Helper functions for building specs

func getStormImage(cluster *stormv1beta1.StormCluster) string {
	registry := cluster.Spec.Image.Registry
	if registry == "" {
		registry = "docker.io"
	}
	return fmt.Sprintf("%s/%s:%s", registry, cluster.Spec.Image.Repository, cluster.Spec.Image.Tag)
}

func getConfigMapName(cluster *stormv1beta1.StormCluster) string {
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.ConfigMap != "" {
		return cluster.Spec.ResourceNames.ConfigMap
	}
	return cluster.Name + "-config"
}

func getImagePullSecrets(cluster *stormv1beta1.StormCluster) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	for _, secret := range cluster.Spec.Image.PullSecrets {
		secrets = append(secrets, corev1.LocalObjectReference{Name: secret})
	}
	return secrets
}

func buildNimbusStatefulSetSpec(cluster *stormv1beta1.StormCluster) appsv1.StatefulSetSpec {
	replicas := cluster.Spec.Nimbus.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "nimbus",
		"cluster":   cluster.Name,
	}

	// Build container
	container := corev1.Container{
		Name:    "nimbus",
		Image:   getStormImage(cluster),
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
			{
				Name:  "STORM_LOG_DIR",
				Value: "/logs",
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
			{
				Name:      "storm-logs",
				MountPath: "/logs",
			},
			{
				Name:      "storm-data",
				MountPath: "/storm/data",
			},
		},
	}

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: getImagePullSecrets(cluster),
		NodeSelector:     cluster.Spec.Nimbus.NodeSelector,
		Tolerations:      cluster.Spec.Nimbus.Tolerations,
		Affinity:         cluster.Spec.Nimbus.Affinity,
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: getConfigMapName(cluster),
						},
					},
				},
			},
			{
				Name: "storm-logs",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
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

	// Build volume claim templates if persistence is enabled
	var volumeClaimTemplates []corev1.PersistentVolumeClaim
	if cluster.Spec.Nimbus.Persistence.Enabled {
		pvcSpec := corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
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
		Replicas: replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		ServiceName: cluster.Name + "-nimbus",
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: podSpec,
		},
		VolumeClaimTemplates: volumeClaimTemplates,
		UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		},
	}
}

func buildSupervisorDeploymentSpec(cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := cluster.Spec.Supervisor.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "supervisor",
		"cluster":   cluster.Name,
	}

	// Build container
	container := corev1.Container{
		Name:      "supervisor",
		Image:     getStormImage(cluster),
		Command:   []string{"storm", "supervisor"},
		Resources: cluster.Spec.Supervisor.Resources,
		Env: append([]corev1.EnvVar{
			{
				Name:  "STORM_CONF_DIR",
				Value: "/conf",
			},
			{
				Name:  "STORM_LOG_DIR",
				Value: "/logs",
			},
		}, cluster.Spec.Supervisor.ExtraEnvVars...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
			{
				Name:      "storm-logs",
				MountPath: "/logs",
			},
			{
				Name:      "storm-data",
				MountPath: "/storm/data",
			},
		},
		Ports: []corev1.ContainerPort{},
	}

	// Add worker ports
	for i := 0; i < int(cluster.Spec.Supervisor.SlotsPerSupervisor); i++ {
		container.Ports = append(container.Ports, corev1.ContainerPort{
			Name:          fmt.Sprintf("worker-%d", i),
			ContainerPort: int32(6700 + i),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	return appsv1.DeploymentSpec{
		Replicas: replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers:       []corev1.Container{container},
				ImagePullSecrets: getImagePullSecrets(cluster),
				NodeSelector:     cluster.Spec.Supervisor.NodeSelector,
				Tolerations:      cluster.Spec.Supervisor.Tolerations,
				Affinity:         cluster.Spec.Supervisor.Affinity,
				Volumes: []corev1.Volume{
					{
						Name: "storm-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: getConfigMapName(cluster),
								},
							},
						},
					},
					{
						Name: "storm-logs",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "storm-data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
	}
}

func buildUIDeploymentSpec(cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := cluster.Spec.UI.Replicas
	labels := map[string]string{
		"app":       "storm",
		"component": "ui",
		"cluster":   cluster.Name,
	}

	// Build container
	container := corev1.Container{
		Name:    "ui",
		Image:   getStormImage(cluster),
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
			{
				Name:  "STORM_LOG_DIR",
				Value: "/logs",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "storm-config",
				MountPath: "/conf",
			},
			{
				Name:      "storm-logs",
				MountPath: "/logs",
			},
			{
				Name:      "storm-data",
				MountPath: "/storm/data",
			},
		},
	}

	return appsv1.DeploymentSpec{
		Replicas: replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers:       []corev1.Container{container},
				ImagePullSecrets: getImagePullSecrets(cluster),
				Volumes: []corev1.Volume{
					{
						Name: "storm-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: getConfigMapName(cluster),
								},
							},
						},
					},
					{
						Name: "storm-logs",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "storm-data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
	}
}
