package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ClusterSpec struct {
	// CloudCredentialSecretName is the id of the secret used to provision
	// the cluster.
	// This field must be in the format of "namespace:name".
	// +kubebuilder:validation:MaxLength=317
	// +nullable
	// +optional
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`

	// KubernetesVersion is the desired version of RKE2/K3s for the cluster.
	// This field is only populated for provisioned and custom clusters.
	// +nullable
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// ClusterAPIConfig is unused.
	// Deprecated: this field is unused and will be removed in a future
	// version.
	// +nullable
	// +optional
	ClusterAPIConfig *ClusterAPIConfig `json:"clusterAPIConfig,omitempty"`

	// RKEConfig represents the desired state for machine configuration and
	// day 2 operations.
	// NOTE: This is only populated for provisioned and custom clusters.
	// +nullable
	// +optional
	RKEConfig *RKEConfig `json:"rkeConfig,omitempty"`

	// LocalClusterAuthEndpoint is the configuration for the local cluster
	// auth endpoint.
	// +optional
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`

	// AgentEnvVars is a list of environment variables that will be set on
	// the cluster agent deployment and system agent service.
	// +nullable
	// +optional
	AgentEnvVars []rkev1.EnvVar `json:"agentEnvVars,omitempty"`

	// ClusterAgentDeploymentCustomization is the customization configuration
	// to apply to the cluster agent deployment.
	// +nullable
	// +optional
	ClusterAgentDeploymentCustomization *AgentDeploymentCustomization `json:"clusterAgentDeploymentCustomization,omitempty"`

	// DefaultPodSecurityAdmissionConfigurationTemplateName is the name of
	// the default psact to use when generating a pod security admissions
	// config file for the cluster.
	// The rancher-webhook will generate a secret containing the rendered
	// AdmissionConfiguration resource as yaml, and create a
	// machineSelectorFile at /etc/rancher/k3s/config/rancher-psact.yaml or
	// /etc/rancher/rke2/config/rancher-psact.yaml for K3s and RKE2
	// respectively.
	// +nullable
	// +optional
	DefaultPodSecurityAdmissionConfigurationTemplateName string `json:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty"`

	// DefaultClusterRoleForProjectMembers is unused.
	// Deprecated: this field is unused and will be removed in a future
	// version.
	// +nullable
	// +optional
	DefaultClusterRoleForProjectMembers string `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`

	// EnableNetworkPolicy defines whether project network isolation is
	// enabled, preventing inter-project communication.
	// It can be used with any network plugin that supports Kubernetes
	// NetworkPolicy enforcement (e.g. canal, calico, cilium).
	// Host network policies are only configured for RKE2 clusters using
	// Calico; other CNIs apply host network policies using pod CIDRs.
	// +nullable
	// +optional
	EnableNetworkPolicy *bool `json:"enableNetworkPolicy,omitempty" norman:"default=false"`

	// FleetAgentDeploymentCustomization is the customization configuration
	// to apply to the fleet agent deployment.
	// +nullable
	// +optional
	FleetAgentDeploymentCustomization *AgentDeploymentCustomization `json:"fleetAgentDeploymentCustomization,omitempty"`

	// RedeploySystemAgentGeneration is used to force the redeployment of the
	// system agent via the system-upgrade controller's system-agent-upgrader
	// plan.
	// NOTE: The cluster-agent must be connected to the Rancher server so the
	// Rancher server can update the system-upgrade-controller plan.
	// +optional
	RedeploySystemAgentGeneration int64 `json:"redeploySystemAgentGeneration,omitempty"`
}

type ClusterAPIConfig struct {
	// ClusterName is unused.
	// Deprecated: this field is unused and will be removed in a future
	// version.
	// +nullable
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}

// Note: if you add new fields to the RKEConfig, please ensure that you check
// `pkg/controllers/provisioningv2/rke2/provisioningcluster/template.go` file and
// drop the fields when saving a copy of the cluster spec on etcd snapshots, otherwise,
// operations using the new fields will cause unnecessary plan thrashing.

type RKEConfig struct {
	rkev1.ClusterConfiguration `json:",inline"`

	// ETCDSnapshotCreate is the configuration for the etcd snapshot creation
	// operation.
	// +nullable
	// +optional
	ETCDSnapshotCreate *rkev1.ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`

	// ETCDSnapshotRestore is the configuration for the etcd snapshot restore
	// operation.
	// +nullable
	// +optional
	ETCDSnapshotRestore *rkev1.ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`

	// RotateCertificates is the configuration for the certificate rotation
	// operation.
	// +nullable
	// +optional
	RotateCertificates *rkev1.RotateCertificates `json:"rotateCertificates,omitempty"`

	// RotateEncryptionKeys is the configuration for the encryption key
	// rotation operation.
	// +nullable
	// +optional
	RotateEncryptionKeys *rkev1.RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// MachinePools is a list of machine pools to be created in the
	// provisioning cluster.
	// +nullable
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=1000
	MachinePools []RKEMachinePool `json:"machinePools,omitempty"`

	// MachinePoolDefaults is the default configuration for machine pools.
	// This configuration will be applied to all machine pools unless
	// overridden by the machine pool configuration.
	// +optional
	MachinePoolDefaults RKEMachinePoolDefaults `json:"machinePoolDefaults,omitempty"`

	// InfrastructureRef is a reference to the infrastructure cluster object
	// that is required when provisioning a CAPI cluster.
	// NOTE: in practice this will always be a rkecluster.rke.cattle.io.
	// +nullable
	// +optional
	InfrastructureRef *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}

// RKEMachinePool is the configuration for a RKE2/K3s machine pool within a provisioning cluster.
type RKEMachinePool struct {
	rkev1.RKECommonNodeConfig `json:",inline"`

	// Paused indicates that the machine pool is paused, preventing CAPI
	// controllers from reconciling it.
	// NOTE: this only applies to the corresponding generated machine
	// deployment object, not the generated machines
	// themselves.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// EtcdRole defines whether the machines provisioned by this pool should
	// be etcd nodes.
	// +optional
	EtcdRole bool `json:"etcdRole,omitempty"`

	// ControlPlaneRole defines whether the machines provisioned by this pool
	// should be controlplane nodes.
	// +optional
	ControlPlaneRole bool `json:"controlPlaneRole,omitempty"`

	// WorkerRole defines whether the machines provisioned by this pool
	// should be worker nodes.
	// +optional
	WorkerRole bool `json:"workerRole,omitempty"`

	// DrainBeforeDelete defines whether the machines provisioned by this
	// pool should be drained prior to deletion.
	// +optional
	DrainBeforeDelete bool `json:"drainBeforeDelete,omitempty"`

	// DrainBeforeDeleteTimeout defines the timeout for draining the machines
	// provisioned by this pool before deletion.
	// +nullable
	// +optional
	DrainBeforeDeleteTimeout *metav1.Duration `json:"drainBeforeDeleteTimeout,omitempty"`

	// NodeConfig is a reference to a MachineConfig object that will be used
	// to configure the machines provisioned by this pool.
	// The NodeConfig object will, in turn, be used to create a corresponding
	// MachineTemplate object for the generated machine deployment.
	// +nullable
	// +required
	NodeConfig *corev1.ObjectReference `json:"machineConfigRef,omitempty"`

	// Name is the internal name of the machine pool.
	// The generated CAPI machine deployment will be a concatenation of the
	// cluster name and the machine pool name which, if over 63 characters is
	// truncated to 54 with a sha256sum appended.
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name,omitempty"`

	// DisplayName is the display name for the generated CAPI
	// machinedeployment object.
	// Deprecated: this field is currently unused and will be removed in a
	// future version.
	// +nullable
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Quantity is the desired number of machines in the machine pool.
	// +kubebuilder:validation:Minimum=0
	// +nullable
	// +optional
	Quantity *int32 `json:"quantity,omitempty"`

	// RollingUpdate is the configuration for the rolling update of the
	// generated machine deployment.
	// +nullable
	// +optional
	RollingUpdate *RKEMachinePoolRollingUpdate `json:"rollingUpdate,omitempty"`

	// MachineDeploymentLabels are the labels to add to the generated
	// machine deployment.
	// +nullable
	// +optional
	MachineDeploymentLabels map[string]string `json:"machineDeploymentLabels,omitempty"`

	// MachineDeploymentAnnotations are the annotations to add to the
	// generated machine deployment.
	// +nullable
	// +optional
	MachineDeploymentAnnotations map[string]string `json:"machineDeploymentAnnotations,omitempty"`

	// NodeStartupTimeout allows setting the maximum time for
	// MachineHealthCheck to consider a Machine unhealthy if a corresponding
	// Node isn't associated through a `Spec.ProviderID` field.
	//
	// The duration set in this field is compared to the greatest of:
	// - Cluster's infrastructure ready condition timestamp (if available)
	// - Control Plane's initialized condition timestamp (if available)
	// - Machine's infrastructure ready condition timestamp (if available)
	// - Machine's metadata creation timestamp
	//
	// Defaults to 10 minutes.
	// If you wish to disable this feature, set the value explicitly to 0.
	// +nullable
	// +optional
	NodeStartupTimeout *metav1.Duration `json:"nodeStartupTimeout,omitempty"`

	// UnhealthyNodeTimeout specifies the maximum duration a generated
	// MachineHealthCheck should wait before marking a not ready machine as
	// unhealthy.
	// +nullable
	// +optional
	UnhealthyNodeTimeout *metav1.Duration `json:"unhealthyNodeTimeout,omitempty"`

	// MaxUnhealthy specifies the minimum number of unhealthy machines that a
	// MachineHealthCheck can tolerate before remediating unhealthy machines.
	// +nullable
	// +optional
	MaxUnhealthy *string `json:"maxUnhealthy,omitempty"`

	// UnhealthyRange specifies the number of unhealthy machines in which a
	// MachineHealthCheck is allowed to remediate.
	// +nullable
	// +optional
	UnhealthyRange *string `json:"unhealthyRange,omitempty"`

	// MachineOS is the operating system of the machines provisioned by this
	// pool.
	// This is only used to designate linux versus windows nodes.
	// +nullable
	// +optional
	MachineOS string `json:"machineOS,omitempty"`

	// DynamicSchemaSpec is a copy of the dynamic schema object's spec field
	// at the time the machine pool was created.
	// Since rancher-machine based MachineTemplates are not api-versioned,
	// this field is used to drop new fields added to the driver if it has
	// been upgraded since initial provisioning.
	// This allows node drivers to be upgraded without triggering a
	// reconciliation of the provisioning cluster.
	// NOTE: This field can only be removed if the
	// "provisioning.cattle.io/allow-dynamic-schema-drop" annotation is
	// present on the provisioning cluster object, otherwise it will be
	// reinserted.
	// +nullable
	// +optional
	DynamicSchemaSpec string `json:"dynamicSchemaSpec,omitempty"`

	// HostnameLengthLimit defines the maximum length of the hostname for
	// machines in this pool.
	// For Windows nodes utilizing NETBIOS authentication, a maximum of 15
	// should be set to ensure all nodes adhere to the protocol's naming
	// requirements.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=63
	// +optional
	HostnameLengthLimit int `json:"hostnameLengthLimit,omitempty"`
}

type RKEMachinePoolRollingUpdate struct {
	// MaxUnavailable is the maximum number of machines that can be
	// unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +nullable
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge is the maximum number of machines that can be scheduled above
	// the desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +nullable
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// RKEMachinePoolDefaults defines the values to set for all machine pools.
// If a value has not been explicitly defined for a machine pool but a default has been set here, then this will be used as a fallback value.
// If a value has been explicitly defined for a machine pool, that value will be used instead.
// NOTE: There is no difference between a zero value and a default value when determining precedence.
type RKEMachinePoolDefaults struct {
	// HostnameLengthLimit defines the maximum length of the hostname for
	// machines in this pool.
	// For Windows nodes utilizing NETBIOS authentication, a maximum of 15
	// should be set to ensure all nodes adhere to the protocol's naming
	// requirements.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=63
	// +optional
	HostnameLengthLimit int `json:"hostnameLengthLimit,omitempty"`
}

// AgentDeploymentCustomization represents the customization options for various agent deployments.
type AgentDeploymentCustomization struct {
	// AppendTolerations is a list of tolerations that will be appended to
	// the agent deployment.
	// +nullable
	// +optional
	AppendTolerations []corev1.Toleration `json:"appendTolerations,omitempty"`

	// OverrideAffinity is an affinity that will be used to override the
	// agent deployment's affinity.
	// +nullable
	// +optional
	OverrideAffinity *corev1.Affinity `json:"overrideAffinity,omitempty"`

	// OverrideResourceRequirements defines the limits, requests, and
	// claims of compute resources for a given container.
	// +nullable
	// +optional
	OverrideResourceRequirements *corev1.ResourceRequirements `json:"overrideResourceRequirements,omitempty"`

	// SchedulingCustomization is an optional configuration that will be
	// used to override the agent deployment's scheduling customization.
	// +nullable
	// +optional
	SchedulingCustomization *AgentSchedulingCustomization `json:"schedulingCustomization,omitempty"`
}

type AgentSchedulingCustomization struct {
	// PriorityClass is the configuration for the priority class associated
	// with the agent deployment.
	// +nullable
	// +optional
	PriorityClass *PriorityClassSpec `json:"priorityClass,omitempty"`

	// PodDisruptionBudget is the configuration for the pod disruption budget
	// associated with the agent deployment.
	// +nullable
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
}

type PriorityClassSpec struct {
	// Value represents the integer value of this priority class.
	// This is the actual priority value that pods receive when their
	// associated deployment specifies the name of this class in the spec.
	// +kubebuilder:validation:Maximum=1000000000
	// +kubebuilder:validation:Minimum=-1000000000
	// +optional
	Value int `json:"value,omitempty"`

	// PreemptionPolicy describes a policy for if/when to preempt a pod.
	// +nullable
	// +optional
	PreemptionPolicy *corev1.PreemptionPolicy `json:"preemptionPolicy,omitempty"`
}

// PodDisruptionBudgetSpec is the spec for the desired pod disruption budget for the corresponding agent deployment.
type PodDisruptionBudgetSpec struct {
	// An eviction is allowed if at least "minAvailable" will still be
	// available after the eviction, i.e. even in the absence of the evicted
	// pod.
	// One can prevent all voluntary evictions by specifying "100%".
	// +kubebuilder:validation:Pattern="^((([1-9]|[1-9][0-9]|100)%)|([1-9][0-9]*|0)|)$"
	// +kubebuilder:validation:MaxLength=10
	// +nullable
	// +optional
	MinAvailable string `json:"minAvailable,omitempty"`

	// An eviction is allowed if at most "maxUnavailable" pods are
	// unavailable after the eviction, i.e. even in the absence of the
	// evicted pod.
	// One can prevent all voluntary evictions by specifying 0.
	// This is a mutually exclusive setting with "minAvailable".
	// +kubebuilder:validation:Pattern="^((([1-9]|[1-9][0-9]|100)%)|([1-9][0-9]*|0)|)$"
	// +kubebuilder:validation:MaxLength=10
	// +nullable
	// +optional
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
}

type ClusterStatus struct {
	// Ready reflects whether the cluster's ready state has previously been
	// reported as true.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Name of the cluster.management.cattle.io object that relates to this
	// cluster.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// FleetWorkspaceName is the name of the fleet workspace that the cluster
	// belongs to.
	// Defaults to the namespace of the cluster object, which is usually
	// "fleet-default".
	// If the provisioningv2-fleet-workspace-back-population feature is
	// enabled and the cluster has the
	// "provisioning.cattle.io/fleet-workspace-name" annotation, this will be
	// set to the value of the annotation.
	// +kubebuilder:validation:MaxLength=63
	// +optional
	FleetWorkspaceName string `json:"fleetWorkspaceName,omitempty"`

	// ClientSecretName is the name of the kubeconfig secret that is used to
	// connect to the cluster.
	// This secret is typically named "<cluster-name>-kubeconfig" and lives
	// in the namespace of the cluster object.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	ClientSecretName string `json:"clientSecretName,omitempty"`

	// AgentDeployed reflects whether the cluster agent has been deployed
	// successfully.
	// +optional
	AgentDeployed bool `json:"agentDeployed,omitempty"`

	// ObservedGeneration is the most recent generation for which the
	// management cluster object was generated for the corresponding
	// provisioning cluster spec.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions is a representation of the Cluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:resource:path=clusters,scope=Namespaced,categories=provisioning
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.kubernetesVersion"
// +kubebuilder:printcolumn:name="Cluster Name",type=string,JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Kubeconfig",type=date,JSONPath=".status.clientSecretName"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.ready"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is the Schema for the provisioning API.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the cluster.
	// +optional
	Spec ClusterSpec `json:"spec,omitempty"`
	// Status is the observed state of the cluster.
	// +optional
	Status ClusterStatus `json:"status,omitempty"`
}
