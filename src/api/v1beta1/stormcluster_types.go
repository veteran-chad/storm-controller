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

// StormClusterSpec defines the desired state of StormCluster
type StormClusterSpec struct {
	// Management mode for the cluster
	// +kubebuilder:validation:Enum=create;reference
	// +kubebuilder:default="create"
	// +optional
	ManagementMode string `json:"managementMode,omitempty"`

	// Resource naming configuration for reference mode
	// +optional
	ResourceNames *ResourceNamesSpec `json:"resourceNames,omitempty"`

	// Image configuration for Storm components
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Nimbus configuration
	// +optional
	Nimbus NimbusSpec `json:"nimbus,omitempty"`

	// Supervisor configuration
	// +optional
	Supervisor SupervisorSpec `json:"supervisor,omitempty"`

	// UI configuration
	// +optional
	UI UISpec `json:"ui,omitempty"`

	// Zookeeper configuration
	// +optional
	Zookeeper ZookeeperSpec `json:"zookeeper,omitempty"`

	// Common Storm configuration parameters
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// Metrics configuration
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`
}

// ResourceNamesSpec defines resource names for reference mode
type ResourceNamesSpec struct {
	// Name of the Nimbus StatefulSet
	// +optional
	NimbusStatefulSet string `json:"nimbusStatefulSet,omitempty"`

	// Name of the Supervisor Deployment
	// +optional
	SupervisorDeployment string `json:"supervisorDeployment,omitempty"`

	// Name of the UI Deployment
	// +optional
	UIDeployment string `json:"uiDeployment,omitempty"`

	// Name of the Nimbus Service
	// +optional
	NimbusService string `json:"nimbusService,omitempty"`

	// Name of the UI Service
	// +optional
	UIService string `json:"uiService,omitempty"`

	// Name of the Storm ConfigMap
	// +optional
	ConfigMap string `json:"configMap,omitempty"`
}

// ImageSpec defines the container image configuration
type ImageSpec struct {
	// Docker registry for the Storm image
	// +kubebuilder:default="docker.io"
	// +optional
	Registry string `json:"registry,omitempty"`

	// Docker repository for the Storm image
	// +kubebuilder:default="apache/storm"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Storm image tag
	// +kubebuilder:default="2.6.0"
	// +optional
	Tag string `json:"tag,omitempty"`

	// Image pull policy
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +kubebuilder:default="IfNotPresent"
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Image pull secrets
	// +optional
	PullSecrets []string `json:"pullSecrets,omitempty"`
}

// NimbusSpec defines the Nimbus configuration
type NimbusSpec struct {
	// Number of Nimbus replicas for HA mode
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Resource requirements for Nimbus pods
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Persistence configuration for Nimbus
	// +optional
	Persistence PersistenceSpec `json:"persistence,omitempty"`

	// Thrift configuration for client connections
	// +optional
	Thrift ThriftSpec `json:"thrift,omitempty"`

	// Extra environment variables for Nimbus
	// +optional
	ExtraEnvVars []corev1.EnvVar `json:"extraEnvVars,omitempty"`

	// Node selector for Nimbus pods
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for Nimbus pods
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for Nimbus pods
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// SupervisorSpec defines the Supervisor configuration
type SupervisorSpec struct {
	// Number of Supervisor replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Deployment mode: "deployment" or "daemonset"
	// +kubebuilder:validation:Enum=deployment;daemonset
	// +kubebuilder:default="deployment"
	// +optional
	DeploymentMode string `json:"deploymentMode,omitempty"`

	// Number of worker slots per supervisor
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=4
	// +optional
	WorkerSlots int32 `json:"workerSlots,omitempty"`

	// Resource requirements for Supervisor pods
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Extra environment variables for Supervisor
	// +optional
	ExtraEnvVars []corev1.EnvVar `json:"extraEnvVars,omitempty"`

	// Node selector for Supervisor pods
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for Supervisor pods
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for Supervisor pods
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// UISpec defines the Storm UI configuration
type UISpec struct {
	// Enable Storm UI
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Number of UI replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Resource requirements for UI pods
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Service configuration
	// +optional
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress configuration
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// Authentication configuration
	// +optional
	Auth *AuthSpec `json:"auth,omitempty"`
}

// ZookeeperSpec defines the Zookeeper configuration
type ZookeeperSpec struct {
	// Enable embedded Zookeeper
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// External Zookeeper servers (if not using embedded)
	// +optional
	ExternalServers []string `json:"externalServers,omitempty"`

	// Chroot path in Zookeeper
	// +kubebuilder:default="/storm"
	// +optional
	ChrootPath string `json:"chrootPath,omitempty"`
}

// PersistenceSpec defines persistence configuration
type PersistenceSpec struct {
	// Enable persistence
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Size of persistent volume
	// +kubebuilder:default="10Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// Storage class for persistent volume
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Access mode for persistent volume
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadOnlyMany;ReadWriteMany
	// +kubebuilder:default="ReadWriteOnce"
	// +optional
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
}

// ServiceSpec defines service configuration
type ServiceSpec struct {
	// Service type
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default="ClusterIP"
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Service port
	// +kubebuilder:default=8080
	// +optional
	Port int32 `json:"port,omitempty"`

	// NodePort (if service type is NodePort)
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`

	// Load balancer IP (if service type is LoadBalancer)
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`

	// Service annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressSpec defines ingress configuration
type IngressSpec struct {
	// Enable ingress
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Ingress class name
	// +kubebuilder:default="nginx"
	// +optional
	ClassName string `json:"className,omitempty"`

	// Hostname for ingress
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Path for ingress
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// Path type for ingress
	// +kubebuilder:validation:Enum=Prefix;Exact;ImplementationSpecific
	// +kubebuilder:default="Prefix"
	// +optional
	PathType string `json:"pathType,omitempty"`

	// TLS configuration
	// +optional
	TLS bool `json:"tls,omitempty"`

	// TLS secret name
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// Ingress annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AuthSpec defines authentication configuration
type AuthSpec struct {
	// Enable authentication
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Authentication type
	// +kubebuilder:validation:Enum=simple;oauth
	// +kubebuilder:default="simple"
	// +optional
	Type string `json:"type,omitempty"`

	// Existing secret with authentication credentials
	// +optional
	ExistingSecret string `json:"existingSecret,omitempty"`

	// Users for simple authentication
	// +optional
	Users []AuthUser `json:"users,omitempty"`
}

// AuthUser defines a user for simple authentication
type AuthUser struct {
	// Username
	Username string `json:"username"`

	// Password (will be stored in a secret)
	Password string `json:"password"`
}

// MetricsSpec defines metrics configuration
type MetricsSpec struct {
	// Enable metrics
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Port for metrics endpoint
	// +kubebuilder:default=7979
	// +optional
	Port int32 `json:"port,omitempty"`

	// Enable ServiceMonitor for Prometheus Operator
	// +kubebuilder:default=false
	// +optional
	ServiceMonitor bool `json:"serviceMonitor,omitempty"`

	// ServiceMonitor labels
	// +optional
	ServiceMonitorLabels map[string]string `json:"serviceMonitorLabels,omitempty"`
}

// ThriftSpec defines Thrift client configuration
type ThriftSpec struct {
	// Thrift port for Nimbus
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6627
	// +optional
	Port int32 `json:"port,omitempty"`

	// Connection timeout in seconds
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	// +optional
	ConnectionTimeout int32 `json:"connectionTimeout,omitempty"`

	// Request timeout in seconds
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=60
	// +optional
	RequestTimeout int32 `json:"requestTimeout,omitempty"`

	// Maximum number of connections in the pool
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=10
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`

	// Minimum number of idle connections in the pool
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=50
	// +kubebuilder:default=2
	// +optional
	MinIdleConnections int32 `json:"minIdleConnections,omitempty"`

	// Maximum idle time for connections in seconds
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:default=300
	// +optional
	MaxIdleTime int32 `json:"maxIdleTime,omitempty"`

	// Maximum retry attempts for failed operations
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	// +optional
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// Retry delay in seconds
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +optional
	RetryDelay int32 `json:"retryDelay,omitempty"`

	// Enable TLS for Thrift connections
	// +kubebuilder:default=false
	// +optional
	UseTLS bool `json:"useTLS,omitempty"`

	// TLS configuration (if TLS is enabled)
	// +optional
	TLS *ThriftTLSSpec `json:"tls,omitempty"`
}

// ThriftTLSSpec defines TLS configuration for Thrift connections
type ThriftTLSSpec struct {
	// Secret containing TLS certificates
	// The secret should contain tls.crt, tls.key, and ca.crt
	CertSecret string `json:"certSecret,omitempty"`

	// Skip TLS verification (insecure)
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// StormClusterStatus defines the observed state of StormCluster
type StormClusterStatus struct {
	// Current phase of the cluster
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Failed;Updating;Terminating
	Phase string `json:"phase,omitempty"`

	// Number of ready Nimbus nodes
	NimbusReady int32 `json:"nimbusReady,omitempty"`

	// Number of ready Supervisor nodes
	SupervisorReady int32 `json:"supervisorReady,omitempty"`

	// Number of ready UI nodes
	UIReady int32 `json:"uiReady,omitempty"`

	// Nimbus leader node
	NimbusLeader string `json:"nimbusLeader,omitempty"`

	// List of all Nimbus nodes
	NimbusNodes []string `json:"nimbusNodes,omitempty"`

	// Total number of worker slots
	TotalSlots int32 `json:"totalSlots,omitempty"`

	// Number of used worker slots
	UsedSlots int32 `json:"usedSlots,omitempty"`

	// Number of free worker slots
	FreeSlots int32 `json:"freeSlots,omitempty"`

	// Formatted slots info for display (used/total)
	SlotsInfo string `json:"slotsInfo,omitempty"`

	// Number of running topologies
	TopologyCount int32 `json:"topologyCount,omitempty"`

	// Cluster endpoints
	Endpoints ClusterEndpoints `json:"endpoints,omitempty"`

	// Last update time
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ClusterEndpoints defines the cluster endpoints
type ClusterEndpoints struct {
	// Nimbus Thrift endpoint
	Nimbus string `json:"nimbus,omitempty"`

	// UI endpoint
	UI string `json:"ui,omitempty"`

	// REST API endpoint
	RestAPI string `json:"restApi,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sc
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Nimbus",type=integer,JSONPath=`.status.nimbusReady`
// +kubebuilder:printcolumn:name="Supervisors",type=integer,JSONPath=`.status.supervisorReady`
// +kubebuilder:printcolumn:name="Slots",type=string,JSONPath=`.status.slotsInfo`,description="Used/Total slots"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StormCluster is the Schema for the stormclusters API
type StormCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StormClusterSpec   `json:"spec,omitempty"`
	Status StormClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StormClusterList contains a list of StormCluster
type StormClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StormCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StormCluster{}, &StormClusterList{})
}
