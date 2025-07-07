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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StormWorkerPoolSpec defines the desired state of StormWorkerPool
type StormWorkerPoolSpec struct {
	// Reference to the StormTopology resource
	// +kubebuilder:validation:Required
	TopologyRef string `json:"topologyRef"`

	// Reference to the StormCluster resource (inherited from topology)
	// +optional
	ClusterRef string `json:"clusterRef,omitempty"`

	// Number of worker replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Override image configuration for workers
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Pod template for workers
	// +optional
	Template *PodTemplateSpec `json:"template,omitempty"`

	// Worker-specific Storm configuration
	// +optional
	WorkerConfig map[string]string `json:"workerConfig,omitempty"`

	// JVM options for workers
	// +optional
	JVMOpts []string `json:"jvmOpts,omitempty"`

	// Port configuration for workers
	// +optional
	Ports *PortConfig `json:"ports,omitempty"`

	// Autoscaling configuration
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
}

// PodTemplateSpec defines the pod template for workers
type PodTemplateSpec struct {
	// Metadata for worker pods
	// +optional
	Metadata *PodMetadata `json:"metadata,omitempty"`

	// Pod spec for workers
	// +optional
	Spec *PodSpecOverride `json:"spec,omitempty"`
}

// PodMetadata defines metadata for worker pods
type PodMetadata struct {
	// Labels for worker pods
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations for worker pods
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodSpecOverride defines pod spec overrides for workers
type PodSpecOverride struct {
	// Container overrides
	// +optional
	Containers []ContainerOverride `json:"containers,omitempty"`

	// Volumes for worker pods
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Affinity rules for worker pods
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations for worker pods
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Node selector for worker pods
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Priority class name for worker pods
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Service account name for worker pods
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Security context for worker pods
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// Use host network
	// +kubebuilder:default=false
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// Use host PID namespace
	// +kubebuilder:default=false
	// +optional
	HostPID bool `json:"hostPID,omitempty"`

	// Use host IPC namespace
	// +kubebuilder:default=false
	// +optional
	HostIPC bool `json:"hostIPC,omitempty"`

	// DNS policy for worker pods
	// +kubebuilder:validation:Enum=ClusterFirst;ClusterFirstWithHostNet;Default;None
	// +optional
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// DNS configuration for worker pods
	// +optional
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`

	// Termination grace period in seconds
	// +kubebuilder:validation:Minimum=0
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// ContainerOverride defines container overrides
type ContainerOverride struct {
	// Container name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volume mounts
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// PortConfig defines port configuration for workers
type PortConfig struct {
	// Starting port number
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6700
	// +optional
	Start int32 `json:"start,omitempty"`

	// Number of ports to allocate
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=4
	// +optional
	Count int32 `json:"count,omitempty"`
}

// StormWorkerPoolStatus defines the observed state of StormWorkerPool
type StormWorkerPoolStatus struct {
	// Current number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// Number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Number of available replicas
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Number of unavailable replicas
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// Number of updated replicas
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Name of the generated deployment
	DeploymentName string `json:"deploymentName,omitempty"`

	// Name of the generated HPA
	HPAName string `json:"hpaName,omitempty"`

	// Current phase of the worker pool
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Updating;Failed;Terminating
	Phase string `json:"phase,omitempty"`

	// Human-readable message about current status
	Message string `json:"message,omitempty"`

	// Last time the status was updated
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=swp
// +kubebuilder:printcolumn:name="Topology",type=string,JSONPath=`.spec.topologyRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StormWorkerPool is the Schema for the stormworkerpools API
type StormWorkerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StormWorkerPoolSpec   `json:"spec,omitempty"`
	Status StormWorkerPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StormWorkerPoolList contains a list of StormWorkerPool
type StormWorkerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StormWorkerPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StormWorkerPool{}, &StormWorkerPoolList{})
}
