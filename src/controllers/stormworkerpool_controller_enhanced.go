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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
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
	workerPoolFinalizer = "storm.apache.org/workerpool-finalizer"
)

// StormWorkerPoolReconcilerEnhanced reconciles a StormWorkerPool object with full functionality
type StormWorkerPoolReconcilerEnhanced struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientManager storm.ClientManager
}

//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormworkerpools/finalizers,verbs=update
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormtopologies,verbs=get;list;watch
//+kubebuilder:rbac:groups=storm.apache.org,resources=stormclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile handles StormWorkerPool reconciliation
func (r *StormWorkerPoolReconcilerEnhanced) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the StormWorkerPool instance
	workerPool := &stormv1beta1.StormWorkerPool{}
	if err := r.Get(ctx, req.NamespacedName, workerPool); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if workerPool.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, workerPool)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(workerPool, workerPoolFinalizer) {
		controllerutil.AddFinalizer(workerPool, workerPoolFinalizer)
		if err := r.Update(ctx, workerPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set initial status if not set
	if workerPool.Status.Phase == "" {
		workerPool.Status.Phase = "Pending"
		if err := r.Status().Update(ctx, workerPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get referenced topology
	topology := &stormv1beta1.StormTopology{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      workerPool.Spec.TopologyRef,
		Namespace: workerPool.Namespace,
	}, topology); err != nil {
		log.Error(err, "Failed to get referenced StormTopology")
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("topology %s not found", workerPool.Spec.TopologyRef))
	}

	// Check if topology is ready
	if topology.Status.Phase != "Running" {
		log.Info("Referenced topology is not running", "topology", topology.Name, "phase", topology.Status.Phase)
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("topology %s is not running (phase: %s)", topology.Name, topology.Status.Phase))
	}

	// Get referenced cluster
	cluster := &stormv1beta1.StormCluster{}
	clusterRef := workerPool.Spec.ClusterRef
	if clusterRef == "" {
		clusterRef = topology.Spec.ClusterRef
	}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      clusterRef,
		Namespace: workerPool.Namespace,
	}, cluster); err != nil {
		log.Error(err, "Failed to get referenced StormCluster")
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("cluster %s not found", clusterRef))
	}

	// Check if cluster is ready
	if cluster.Status.Phase != "Running" {
		log.Info("Referenced cluster is not running", "cluster", cluster.Name, "phase", cluster.Status.Phase)
		return r.updateStatusError(ctx, workerPool, fmt.Errorf("cluster %s is not running (phase: %s)", cluster.Name, cluster.Status.Phase))
	}

	// Reconcile worker deployment
	deployment, err := r.reconcileDeployment(ctx, workerPool, topology, cluster)
	if err != nil {
		log.Error(err, "Failed to reconcile deployment")
		return r.updateStatusError(ctx, workerPool, err)
	}

	// Setup HPA if autoscaling is enabled
	if workerPool.Spec.Autoscaling != nil && workerPool.Spec.Autoscaling.Enabled {
		if err := r.reconcileHPA(ctx, workerPool, deployment); err != nil {
			log.Error(err, "Failed to reconcile HPA")
			return r.updateStatusError(ctx, workerPool, err)
		}
	} else {
		// Remove HPA if autoscaling is disabled
		if err := r.deleteHPA(ctx, workerPool); err != nil {
			log.Error(err, "Failed to delete HPA")
		}
	}

	// Create headless service for worker discovery
	if err := r.reconcileService(ctx, workerPool); err != nil {
		log.Error(err, "Failed to reconcile service")
		return r.updateStatusError(ctx, workerPool, err)
	}

	// Update status
	if err := r.updateStatus(ctx, workerPool, deployment); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to update status periodically
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileDeployment creates or updates the worker deployment
func (r *StormWorkerPoolReconcilerEnhanced) reconcileDeployment(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) (*appsv1.Deployment, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("%s-workers", workerPool.Name)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, deployment, r.Scheme); err != nil {
			return err
		}

		// Build deployment spec
		deployment.Spec = r.buildWorkerDeploymentSpec(workerPool, topology, cluster)

		// Update deployment name in status
		workerPool.Status.DeploymentName = deploymentName

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Info("Reconciled worker deployment", "name", deployment.Name)
	return deployment, nil
}

// buildWorkerDeploymentSpec builds the worker deployment specification
func (r *StormWorkerPoolReconcilerEnhanced) buildWorkerDeploymentSpec(workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) appsv1.DeploymentSpec {
	replicas := workerPool.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	labels := map[string]string{
		"app":        "storm",
		"component":  "worker",
		"cluster":    cluster.Name,
		"topology":   topology.Name,
		"workerpool": workerPool.Name,
	}

	// Add custom labels from pod template
	if workerPool.Spec.Template != nil && workerPool.Spec.Template.Metadata != nil {
		for k, v := range workerPool.Spec.Template.Metadata.Labels {
			labels[k] = v
		}
	}

	// Build container
	container := r.buildWorkerContainer(workerPool, topology, cluster)

	// Build pod spec
	podSpec := r.buildWorkerPodSpec(workerPool, cluster, container)

	// Apply pod spec overrides
	if workerPool.Spec.Template != nil && workerPool.Spec.Template.Spec != nil {
		r.applyPodSpecOverrides(podSpec, workerPool.Spec.Template.Spec)
	}

	// Build pod template
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: *podSpec,
	}

	// Add custom annotations from pod template
	if workerPool.Spec.Template != nil && workerPool.Spec.Template.Metadata != nil && len(workerPool.Spec.Template.Metadata.Annotations) > 0 {
		podTemplate.ObjectMeta.Annotations = workerPool.Spec.Template.Metadata.Annotations
	}

	return appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"workerpool": workerPool.Name,
			},
		},
		Template: podTemplate,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "25%",
				},
				MaxSurge: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "25%",
				},
			},
		},
	}
}

// buildWorkerContainer builds the worker container specification
func (r *StormWorkerPoolReconcilerEnhanced) buildWorkerContainer(workerPool *stormv1beta1.StormWorkerPool, topology *stormv1beta1.StormTopology, cluster *stormv1beta1.StormCluster) corev1.Container {
	// Determine image
	image := r.getStormImage(cluster)
	if workerPool.Spec.Image != nil {
		// Override with worker pool specific image
		image = r.getImageFromSpec(workerPool.Spec.Image)
	}

	// Build ports
	ports := []corev1.ContainerPort{}
	portStart := int32(6700)
	portCount := int32(4)

	if workerPool.Spec.Ports != nil {
		if workerPool.Spec.Ports.Start > 0 {
			portStart = workerPool.Spec.Ports.Start
		}
		if workerPool.Spec.Ports.Count > 0 {
			portCount = workerPool.Spec.Ports.Count
		}
	}

	for i := int32(0); i < portCount; i++ {
		ports = append(ports, corev1.ContainerPort{
			Name:          fmt.Sprintf("worker-%d", i),
			ContainerPort: portStart + i,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Build environment variables
	env := []corev1.EnvVar{
		{
			Name:  "STORM_CONF_DIR",
			Value: "/conf",
		},
		{
			Name:  "TOPOLOGY_NAME",
			Value: topology.Spec.Topology.Name,
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}

	// Add JVM opts if specified
	if len(workerPool.Spec.JVMOpts) > 0 {
		jvmOpts := ""
		for _, opt := range workerPool.Spec.JVMOpts {
			jvmOpts += opt + " "
		}
		env = append(env, corev1.EnvVar{
			Name:  "STORM_WORKER_CHILDOPTS",
			Value: jvmOpts,
		})
	}

	// Default resources if not specified
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	}

	container := corev1.Container{
		Name:      "worker",
		Image:     image,
		Command:   []string{"storm", "supervisor"},
		Ports:     ports,
		Env:       env,
		Resources: resources,
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
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(int(portStart)),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(int(portStart)),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
	}

	return container
}

// buildWorkerPodSpec builds the worker pod specification
func (r *StormWorkerPoolReconcilerEnhanced) buildWorkerPodSpec(workerPool *stormv1beta1.StormWorkerPool, cluster *stormv1beta1.StormCluster, container corev1.Container) *corev1.PodSpec {
	// Get image pull secrets
	imagePullSecrets := r.getImagePullSecrets(cluster)
	if workerPool.Spec.Image != nil && len(workerPool.Spec.Image.PullSecrets) > 0 {
		// Override with worker pool specific secrets
		imagePullSecrets = []corev1.LocalObjectReference{}
		for _, secret := range workerPool.Spec.Image.PullSecrets {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
	}

	// Determine configmap name based on cluster's management mode
	configMapName := stormConfigName
	if cluster.Spec.ManagementMode == "reference" && cluster.Spec.ResourceNames != nil && cluster.Spec.ResourceNames.ConfigMap != "" {
		configMapName = cluster.Spec.ResourceNames.ConfigMap
	}

	podSpec := &corev1.PodSpec{
		Containers:       []corev1.Container{container},
		ImagePullSecrets: imagePullSecrets,
		Volumes: []corev1.Volume{
			{
				Name: "storm-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
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

	return podSpec
}

// applyPodSpecOverrides applies pod spec overrides from the worker pool
func (r *StormWorkerPoolReconcilerEnhanced) applyPodSpecOverrides(podSpec *corev1.PodSpec, overrides *stormv1beta1.PodSpecOverride) {
	// Apply container overrides
	for _, containerOverride := range overrides.Containers {
		for i, container := range podSpec.Containers {
			if container.Name == containerOverride.Name {
				// Apply resource overrides
				if len(containerOverride.Resources.Requests) > 0 || len(containerOverride.Resources.Limits) > 0 {
					podSpec.Containers[i].Resources = containerOverride.Resources
				}
				// Apply env overrides
				if len(containerOverride.Env) > 0 {
					podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, containerOverride.Env...)
				}
				// Apply volume mount overrides
				if len(containerOverride.VolumeMounts) > 0 {
					podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, containerOverride.VolumeMounts...)
				}
			}
		}
	}

	// Apply pod-level overrides
	if len(overrides.Volumes) > 0 {
		podSpec.Volumes = append(podSpec.Volumes, overrides.Volumes...)
	}
	if overrides.Affinity != nil {
		podSpec.Affinity = overrides.Affinity
	}
	if len(overrides.Tolerations) > 0 {
		podSpec.Tolerations = overrides.Tolerations
	}
	if len(overrides.NodeSelector) > 0 {
		podSpec.NodeSelector = overrides.NodeSelector
	}
	if overrides.PriorityClassName != "" {
		podSpec.PriorityClassName = overrides.PriorityClassName
	}
	if overrides.ServiceAccountName != "" {
		podSpec.ServiceAccountName = overrides.ServiceAccountName
	}
	if overrides.SecurityContext != nil {
		podSpec.SecurityContext = overrides.SecurityContext
	}
	if overrides.HostNetwork {
		podSpec.HostNetwork = true
	}
	if overrides.HostPID {
		podSpec.HostPID = true
	}
	if overrides.HostIPC {
		podSpec.HostIPC = true
	}
	if overrides.DNSPolicy != "" {
		podSpec.DNSPolicy = overrides.DNSPolicy
	}
	if overrides.DNSConfig != nil {
		podSpec.DNSConfig = overrides.DNSConfig
	}
	if overrides.TerminationGracePeriodSeconds != nil {
		podSpec.TerminationGracePeriodSeconds = overrides.TerminationGracePeriodSeconds
	}
}

// reconcileHPA creates or updates the HorizontalPodAutoscaler
func (r *StormWorkerPoolReconcilerEnhanced) reconcileHPA(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, deployment *appsv1.Deployment) error {
	log := log.FromContext(ctx)

	hpaName := fmt.Sprintf("%s-hpa", workerPool.Name)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, hpa, r.Scheme); err != nil {
			return err
		}

		// Build HPA spec
		hpa.Spec = r.buildHPASpec(workerPool, deployment)

		// Update HPA name in status
		workerPool.Status.HPAName = hpaName

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled HPA", "name", hpa.Name)
	return nil
}

// buildHPASpec builds the HPA specification
func (r *StormWorkerPoolReconcilerEnhanced) buildHPASpec(workerPool *stormv1beta1.StormWorkerPool, deployment *appsv1.Deployment) autoscalingv2.HorizontalPodAutoscalerSpec {
	minReplicas := workerPool.Spec.Autoscaling.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}

	maxReplicas := workerPool.Spec.Autoscaling.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = 10
	}

	spec := autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
		},
		MinReplicas: &minReplicas,
		MaxReplicas: maxReplicas,
		Metrics:     []autoscalingv2.MetricSpec{},
	}

	// Add CPU metric if specified
	if workerPool.Spec.Autoscaling.TargetCPUUtilizationPercentage != nil {
		spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: workerPool.Spec.Autoscaling.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	// Add memory metric if specified
	if workerPool.Spec.Autoscaling.TargetMemoryUtilizationPercentage != nil {
		spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: workerPool.Spec.Autoscaling.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	// Add custom metrics
	for _, customMetric := range workerPool.Spec.Autoscaling.CustomMetrics {
		switch customMetric.Type {
		case "pods":
			quantity, _ := resource.ParseQuantity(customMetric.TargetAverageValue)
			spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
				Type: autoscalingv2.PodsMetricSourceType,
				Pods: &autoscalingv2.PodsMetricSource{
					Metric: autoscalingv2.MetricIdentifier{
						Name: customMetric.Name,
					},
					Target: autoscalingv2.MetricTarget{
						Type:         autoscalingv2.AverageValueMetricType,
						AverageValue: &quantity,
					},
				},
			})
		case "external":
			quantity, _ := resource.ParseQuantity(customMetric.TargetValue)
			spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
				Type: autoscalingv2.ExternalMetricSourceType,
				External: &autoscalingv2.ExternalMetricSource{
					Metric: autoscalingv2.MetricIdentifier{
						Name: customMetric.Name,
					},
					Target: autoscalingv2.MetricTarget{
						Type:  autoscalingv2.ValueMetricType,
						Value: &quantity,
					},
				},
			})
		}
	}

	// Set default behavior
	spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{
		ScaleDown: &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: &[]int32{300}[0], // 5 minutes
			Policies: []autoscalingv2.HPAScalingPolicy{
				{
					Type:          autoscalingv2.PercentScalingPolicy,
					Value:         10,
					PeriodSeconds: 60,
				},
			},
		},
		ScaleUp: &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: &[]int32{60}[0], // 1 minute
			Policies: []autoscalingv2.HPAScalingPolicy{
				{
					Type:          autoscalingv2.PercentScalingPolicy,
					Value:         100,
					PeriodSeconds: 60,
				},
				{
					Type:          autoscalingv2.PodsScalingPolicy,
					Value:         4,
					PeriodSeconds: 60,
				},
			},
			SelectPolicy: &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MaxChangePolicySelect}[0],
		},
	}

	return spec
}

// deleteHPA removes the HPA if it exists
func (r *StormWorkerPoolReconcilerEnhanced) deleteHPA(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	if workerPool.Status.HPAName == "" {
		return nil
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workerPool.Status.HPAName,
			Namespace: workerPool.Namespace,
		},
	}

	if err := r.Delete(ctx, hpa); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// Clear HPA name from status
	workerPool.Status.HPAName = ""

	return nil
}

// reconcileService creates or updates the headless service for worker discovery
func (r *StormWorkerPoolReconcilerEnhanced) reconcileService(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) error {
	log := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-workers", workerPool.Name),
			Namespace: workerPool.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set owner reference
		if err := controllerutil.SetControllerReference(workerPool, service, r.Scheme); err != nil {
			return err
		}

		// Build service spec
		service.Spec = corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone, // Headless service
			Selector: map[string]string{
				"workerpool": workerPool.Name,
			},
			Ports: []corev1.ServicePort{},
		}

		// Add ports based on configuration
		portStart := int32(6700)
		portCount := int32(4)

		if workerPool.Spec.Ports != nil {
			if workerPool.Spec.Ports.Start > 0 {
				portStart = workerPool.Spec.Ports.Start
			}
			if workerPool.Spec.Ports.Count > 0 {
				portCount = workerPool.Spec.Ports.Count
			}
		}

		for i := int32(0); i < portCount; i++ {
			service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
				Name:       fmt.Sprintf("worker-%d", i),
				Port:       portStart + i,
				TargetPort: intstr.FromInt(int(portStart + i)),
				Protocol:   corev1.ProtocolTCP,
			})
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("Reconciled worker service", "name", service.Name)
	return nil
}

// updateStatus updates the worker pool status
func (r *StormWorkerPoolReconcilerEnhanced) updateStatus(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, deployment *appsv1.Deployment) error {
	// Get deployment status
	deployment = &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      workerPool.Status.DeploymentName,
		Namespace: workerPool.Namespace,
	}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// Deployment not found
		workerPool.Status.Phase = "Failed"
		workerPool.Status.Message = "Deployment not found"
	} else {
		// Update replica counts
		workerPool.Status.Replicas = deployment.Status.Replicas
		workerPool.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		workerPool.Status.AvailableReplicas = deployment.Status.AvailableReplicas
		workerPool.Status.UnavailableReplicas = deployment.Status.UnavailableReplicas
		workerPool.Status.UpdatedReplicas = deployment.Status.UpdatedReplicas

		// Determine phase
		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			workerPool.Status.Phase = "Running"
			workerPool.Status.Message = fmt.Sprintf("All %d worker(s) are ready", deployment.Status.ReadyReplicas)
		} else if deployment.Status.ReadyReplicas > 0 {
			workerPool.Status.Phase = "Updating"
			workerPool.Status.Message = fmt.Sprintf("%d of %d worker(s) are ready", deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
		} else {
			workerPool.Status.Phase = "Creating"
			workerPool.Status.Message = "Waiting for workers to become ready"
		}
	}

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

	metrics.StormWorkerPoolReplicas.With(map[string]string{
		"pool":      workerPool.Name,
		"namespace": workerPool.Namespace,
		"topology":  workerPool.Spec.TopologyRef,
		"state":     "available",
	}).Set(float64(workerPool.Status.AvailableReplicas))

	// Update conditions
	if workerPool.Status.Phase == "Running" {
		meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: workerPool.Generation,
			Reason:             "WorkersReady",
			Message:            workerPool.Status.Message,
		})
	} else {
		meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: workerPool.Generation,
			Reason:             "WorkersNotReady",
			Message:            workerPool.Status.Message,
		})
	}

	// Check autoscaling status
	if workerPool.Spec.Autoscaling != nil && workerPool.Spec.Autoscaling.Enabled {
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      workerPool.Status.HPAName,
			Namespace: workerPool.Namespace,
		}, hpa); err == nil {
			meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
				Type:               "Autoscaling",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: workerPool.Generation,
				Reason:             "HPAActive",
				Message:            fmt.Sprintf("HPA is active (current: %d, min: %d, max: %d)", hpa.Status.CurrentReplicas, *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas),
			})
		}
	}

	workerPool.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	return r.Status().Update(ctx, workerPool)
}

// updateStatusError updates the status with an error
func (r *StormWorkerPoolReconcilerEnhanced) updateStatusError(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool, err error) (ctrl.Result, error) {
	workerPool.Status.Phase = "Failed"
	workerPool.Status.Message = err.Error()
	workerPool.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	meta.SetStatusCondition(&workerPool.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: workerPool.Generation,
		Reason:             "ReconciliationFailed",
		Message:            err.Error(),
	})

	if statusErr := r.Status().Update(ctx, workerPool); statusErr != nil {
		return ctrl.Result{}, statusErr
	}

	// Retry after delay
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleDeletion handles worker pool deletion
func (r *StormWorkerPoolReconcilerEnhanced) handleDeletion(ctx context.Context, workerPool *stormv1beta1.StormWorkerPool) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(workerPool, workerPoolFinalizer) {
		log.Info("Handling worker pool deletion", "workerpool", workerPool.Name)

		// Cleanup any worker pool specific resources if needed
		// For now, Kubernetes will handle deletion of owned resources

		// Remove finalizer
		controllerutil.RemoveFinalizer(workerPool, workerPoolFinalizer)
		if err := r.Update(ctx, workerPool); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// Helper functions

func (r *StormWorkerPoolReconcilerEnhanced) getStormImage(cluster *stormv1beta1.StormCluster) string {
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

func (r *StormWorkerPoolReconcilerEnhanced) getImageFromSpec(imageSpec *stormv1beta1.ImageSpec) string {
	registry := imageSpec.Registry
	if registry == "" {
		registry = "docker.io"
	}
	repository := imageSpec.Repository
	if repository == "" {
		repository = "apache/storm"
	}
	tag := imageSpec.Tag
	if tag == "" {
		tag = "2.6.0"
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func (r *StormWorkerPoolReconcilerEnhanced) getImagePullSecrets(cluster *stormv1beta1.StormCluster) []corev1.LocalObjectReference {
	secrets := []corev1.LocalObjectReference{}
	for _, secret := range cluster.Spec.Image.PullSecrets {
		secrets = append(secrets, corev1.LocalObjectReference{Name: secret})
	}
	return secrets
}

// findWorkerPoolsForCluster returns requests for all worker pools that reference this cluster
func (r *StormWorkerPoolReconcilerEnhanced) findWorkerPoolsForCluster(ctx context.Context, obj client.Object) []ctrl.Request {
	cluster := obj.(*stormv1beta1.StormCluster)

	// Find all worker pools in the same namespace
	workerPoolList := &stormv1beta1.StormWorkerPoolList{}
	if err := r.List(ctx, workerPoolList, client.InNamespace(cluster.Namespace)); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, workerPool := range workerPoolList.Items {
		// Check if this worker pool references the cluster directly
		if workerPool.Spec.ClusterRef == cluster.Name {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			})
			continue
		}

		// Check if this worker pool references a topology that references this cluster
		if workerPool.Spec.TopologyRef != "" {
			topology := &stormv1beta1.StormTopology{}
			if err := r.Get(ctx, client.ObjectKey{
				Name:      workerPool.Spec.TopologyRef,
				Namespace: workerPool.Namespace,
			}, topology); err == nil && topology.Spec.ClusterRef == cluster.Name {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      workerPool.Name,
						Namespace: workerPool.Namespace,
					},
				})
			}
		}
	}

	return requests
}

// findWorkerPoolsForTopology returns requests for all worker pools that reference this topology
func (r *StormWorkerPoolReconcilerEnhanced) findWorkerPoolsForTopology(ctx context.Context, obj client.Object) []ctrl.Request {
	topology := obj.(*stormv1beta1.StormTopology)

	workerPoolList := &stormv1beta1.StormWorkerPoolList{}
	if err := r.List(ctx, workerPoolList, client.InNamespace(topology.Namespace)); err != nil {
		return nil
	}

	requests := []ctrl.Request{}
	for _, workerPool := range workerPoolList.Items {
		if workerPool.Spec.TopologyRef == topology.Name {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      workerPool.Name,
					Namespace: workerPool.Namespace,
				},
			})
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager
func (r *StormWorkerPoolReconcilerEnhanced) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&stormv1beta1.StormWorkerPool{}).
		Owns(&appsv1.Deployment{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&corev1.Service{}).
		Watches(&stormv1beta1.StormTopology{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkerPoolsForTopology)).
		Watches(&stormv1beta1.StormCluster{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkerPoolsForCluster)).
		Complete(r)
}
