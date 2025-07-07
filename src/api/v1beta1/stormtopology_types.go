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

// StormTopologySpec defines the desired state of StormTopology
type StormTopologySpec struct {
	// Reference to the StormCluster resource
	// +kubebuilder:validation:Required
	ClusterRef string `json:"clusterRef"`

	// Topology configuration
	// +kubebuilder:validation:Required
	Topology TopologySpec `json:"topology"`

	// Worker configuration
	// +optional
	Workers *WorkersSpec `json:"workers,omitempty"`

	// Lifecycle configuration
	// +optional
	Lifecycle *LifecycleSpec `json:"lifecycle,omitempty"`

	// Suspend will suspend the topology if set to true
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// TopologySpec defines the topology configuration
type TopologySpec struct {
	// Name of the topology
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// JAR file configuration
	// +kubebuilder:validation:Required
	Jar JarSpec `json:"jar"`

	// Main class to execute
	// +optional
	MainClass string `json:"mainClass,omitempty"`

	// Arguments to pass to the topology
	// +optional
	Args []string `json:"args,omitempty"`

	// Topology-specific configuration
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// JarSpec defines JAR file configuration
type JarSpec struct {
	// URL to download JAR from
	// +optional
	URL string `json:"url,omitempty"`

	// ConfigMap containing JAR file
	// +optional
	ConfigMap string `json:"configMap,omitempty"`

	// Secret containing JAR file
	// +optional
	Secret string `json:"secret,omitempty"`

	// S3 location for JAR file
	// +optional
	S3 *S3Location `json:"s3,omitempty"`

	// Container image containing JAR file
	// +optional
	Container *ContainerJarSource `json:"container,omitempty"`
}

// S3Location defines S3 location for JAR file
type S3Location struct {
	// S3 bucket name
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// S3 object key
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// AWS region
	// +optional
	Region string `json:"region,omitempty"`

	// S3 endpoint (for S3-compatible storage)
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Secret containing AWS credentials
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

// ContainerJarSource defines container-based JAR source
type ContainerJarSource struct {
	// Container image containing the JAR file
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Path to JAR file inside container
	// +kubebuilder:default="/app/topology.jar"
	// +optional
	Path string `json:"path,omitempty"`

	// Image pull policy
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Secrets for pulling images from private registries
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`

	// Resource requirements for the JAR extraction container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Security context for the JAR extraction container
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Environment variables for the JAR extraction container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volume mounts for the JAR extraction container
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// JAR extraction strategy
	// +kubebuilder:validation:Enum=initContainer;job;sidecar
	// +kubebuilder:default=initContainer
	// +optional
	ExtractionMode string `json:"extractionMode,omitempty"`

	// Timeout for JAR extraction process
	// +kubebuilder:default=300
	// +optional
	ExtractionTimeoutSeconds *int32 `json:"extractionTimeoutSeconds,omitempty"`

	// Checksum verification for JAR file
	// +optional
	Checksum *ChecksumSpec `json:"checksum,omitempty"`
}

// ChecksumSpec defines checksum verification
type ChecksumSpec struct {
	// Algorithm used for checksum (sha256, sha512, md5)
	// +kubebuilder:validation:Enum=sha256;sha512;md5
	// +kubebuilder:default=sha256
	Algorithm string `json:"algorithm"`

	// Expected checksum value
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// Path to checksum file inside container (alternative to value)
	// +optional
	File string `json:"file,omitempty"`
}

// WorkersSpec defines worker configuration
type WorkersSpec struct {
	// Number of worker replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Resource requirements for worker pods
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// JVM options for workers
	// +optional
	JVMOpts []string `json:"jvmOpts,omitempty"`

	// Autoscaling configuration
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Node selector for worker pods
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for worker pods
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity rules for worker pods
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations for worker pods
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Labels for worker pods
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`
}

// AutoscalingSpec defines autoscaling configuration
type AutoscalingSpec struct {
	// Enable autoscaling
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Minimum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// Maximum number of replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// Target CPU utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=80
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// Target memory utilization percentage
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`

	// Custom metrics for autoscaling
	// +optional
	CustomMetrics []CustomMetric `json:"customMetrics,omitempty"`

	// Metrics for autoscaling (deprecated, use CustomMetrics)
	// +optional
	Metrics []MetricSpec `json:"metrics,omitempty"`

	// Scaling behavior configuration
	// +optional
	Behavior *ScalingBehavior `json:"behavior,omitempty"`
}

// CustomMetric defines a custom metric for autoscaling
type CustomMetric struct {
	// Name of the metric
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the metric (pods, resource, external)
	// +kubebuilder:validation:Enum=pods;resource;external
	// +kubebuilder:default="pods"
	// +optional
	Type string `json:"type,omitempty"`

	// Target value for the metric
	// +kubebuilder:validation:Required
	TargetValue string `json:"targetValue"`

	// Target average value for the metric
	// +optional
	TargetAverageValue string `json:"targetAverageValue,omitempty"`
}

// MetricSpec defines a metric for autoscaling
type MetricSpec struct {
	// Type of metric
	// +kubebuilder:validation:Enum=cpu;memory;pending-tuples;capacity;latency
	Type string `json:"type"`

	// Target value for the metric
	Target MetricTarget `json:"target"`
}

// MetricTarget defines the target value for a metric
type MetricTarget struct {
	// Target average utilization (for cpu/memory)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`

	// Target average value
	// +optional
	AverageValue string `json:"averageValue,omitempty"`

	// Target value
	// +optional
	Value string `json:"value,omitempty"`
}

// ScalingBehavior defines scaling behavior
type ScalingBehavior struct {
	// Scale down behavior
	// +optional
	ScaleDown *ScalePolicy `json:"scaleDown,omitempty"`

	// Scale up behavior
	// +optional
	ScaleUp *ScalePolicy `json:"scaleUp,omitempty"`
}

// ScalePolicy defines a scaling policy
type ScalePolicy struct {
	// Stabilization window in seconds
	// +kubebuilder:validation:Minimum=0
	// +optional
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds,omitempty"`

	// Scaling policies
	// +optional
	Policies []ScalingPolicyRule `json:"policies,omitempty"`
}

// ScalingPolicyRule defines a scaling policy rule
type ScalingPolicyRule struct {
	// Type of scaling
	// +kubebuilder:validation:Enum=Percent;Pods
	Type string `json:"type"`

	// Value for scaling
	// +kubebuilder:validation:Minimum=1
	Value int32 `json:"value"`

	// Period in seconds
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds"`
}

// LifecycleSpec defines lifecycle configuration
type LifecycleSpec struct {
	// Kill wait time in seconds
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=30
	// +optional
	KillWaitSeconds int32 `json:"killWaitSeconds,omitempty"`

	// Update strategy
	// +kubebuilder:validation:Enum=recreate;rolling
	// +kubebuilder:default="recreate"
	// +optional
	UpdateStrategy string `json:"updateStrategy,omitempty"`

	// Pre-stop hook configuration
	// +optional
	PreStop *HookSpec `json:"preStop,omitempty"`

	// Post-start hook configuration
	// +optional
	PostStart *HookSpec `json:"postStart,omitempty"`
}

// HookSpec defines a lifecycle hook
type HookSpec struct {
	// Exec specifies the action to take
	// +optional
	Exec *corev1.ExecAction `json:"exec,omitempty"`

	// HTTPGet specifies the http request to perform
	// +optional
	HTTPGet *corev1.HTTPGetAction `json:"httpGet,omitempty"`

	// TCPSocket specifies an action involving a TCP port
	// +optional
	TCPSocket *corev1.TCPSocketAction `json:"tcpSocket,omitempty"`
}

// StormTopologyStatus defines the observed state of StormTopology
type StormTopologyStatus struct {
	// Current phase of the topology
	// +kubebuilder:validation:Enum=Pending;Submitted;Running;Suspended;Failed;Killed;Updating
	Phase string `json:"phase,omitempty"`

	// Storm-assigned topology ID
	TopologyID string `json:"topologyId,omitempty"`

	// Storm-assigned topology name (may differ from metadata.name)
	TopologyName string `json:"topologyName,omitempty"`

	// Number of workers currently allocated
	Workers int32 `json:"workers,omitempty"`

	// Number of executors
	Executors int32 `json:"executors,omitempty"`

	// Number of tasks
	Tasks int32 `json:"tasks,omitempty"`

	// Uptime in human-readable format
	Uptime string `json:"uptime,omitempty"`

	// Topology metrics
	// +optional
	Metrics *TopologyMetrics `json:"metrics,omitempty"`

	// Last error message
	LastError string `json:"lastError,omitempty"`

	// Last update time
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Deployed topology version
	// +optional
	DeployedVersion string `json:"deployedVersion,omitempty"`

	// Conditions
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// TopologyMetrics defines topology performance metrics
type TopologyMetrics struct {
	// Total number of emitted tuples
	Emitted int64 `json:"emitted,omitempty"`

	// Total number of transferred tuples
	Transferred int64 `json:"transferred,omitempty"`

	// Total number of acked tuples
	Acked int64 `json:"acked,omitempty"`

	// Total number of failed tuples
	Failed int64 `json:"failed,omitempty"`

	// Average complete latency in milliseconds
	CompleteLatencyMs int64 `json:"completeLatencyMs,omitempty"`

	// Number of pending tuples
	PendingTuples int64 `json:"pendingTuples,omitempty"`

	// Capacity percentage (0-100)
	CapacityPercent int32 `json:"capacityPercent,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=st
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Workers",type=integer,JSONPath=`.status.workers`
// +kubebuilder:printcolumn:name="Uptime",type=string,JSONPath=`.status.uptime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StormTopology is the Schema for the stormtopologies API
type StormTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StormTopologySpec   `json:"spec,omitempty"`
	Status StormTopologyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StormTopologyList contains a list of StormTopology
type StormTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StormTopology `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StormTopology{}, &StormTopologyList{})
}
