package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=clusters,scope=Namespaced,categories=provisioning
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.rkeConfig.kubernetesVersion"
// +kubebuilder:printcolumn:name="Cluster Name",type=string,JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.ready"
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is the Schema for the provisioning API.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the cluster.
	Spec ClusterSpec `json:"spec"`
	// Status is the observed state of the cluster.
	// +optional
	Status ClusterStatus `json:"status,omitempty"`
}

type ClusterSpec struct {
	// CloudCredentialSecretName is the id of the secret used to provision the cluster.
	// This field must be in the format of "namespace:name".
	// +optional
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`

	// KubernetesVersion is the desired version of RKE2/K3s for the cluster.
	// This field is only populated for provisioned and custom clusters.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// +optional
	ClusterAPIConfig *ClusterAPIConfig `json:"clusterAPIConfig,omitempty"`
	// RKEConfig represents the desired state for machine configuration and day 2 operations.
	// NOTE: This is only populated for provisioned and custom clusters.
	// +optional
	RKEConfig *RKEConfig `json:"rkeConfig,omitempty"`
	// LocalClusterAuthEndpoint is an optional field that can be used to configure the local cluster auth endpoint.
	// +optional
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`

	// AgentEnvVars is a list of environment variables that will be set on the cluster agent deployment and system agent service.
	// +optional
	AgentEnvVars []rkev1.EnvVar `json:"agentEnvVars,omitempty"`
	// ClusterAgentDeploymentCustomization is the customization configuration to apply to the cluster agent deployment.
	// +optional
	ClusterAgentDeploymentCustomization *AgentDeploymentCustomization `json:"clusterAgentDeploymentCustomization,omitempty"`
	// +optional
	DefaultPodSecurityAdmissionConfigurationTemplateName string `json:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty"`
	// +optional
	DefaultClusterRoleForProjectMembers string `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	// +optional
	EnableNetworkPolicy *bool `json:"enableNetworkPolicy,omitempty" norman:"default=false"`
	// ClusterAgentDeploymentCustomization is the customization configuration to apply to the fleet agent deployment.
	// +optional
	FleetAgentDeploymentCustomization *AgentDeploymentCustomization `json:"fleetAgentDeploymentCustomization,omitempty"`

	// RedeploySystemAgentGeneration is used to force the redeployment of the system agent via
	// the system-upgrade controller's system-agent-upgrader plan.
	// NOTE: The cluster-agent must be connected to the Rancher server so the Rancher server can update the
	// system-upgrade-controller plan.
	// +optional
	RedeploySystemAgentGeneration int64 `json:"redeploySystemAgentGeneration,omitempty"`
}

// AgentDeploymentCustomization represents the customization options for various agent deployments.
type AgentDeploymentCustomization struct {
	// AppendTolerations is a list of tolerations that will be appended to the agent deployment.
	// +optional
	AppendTolerations []v1.Toleration `json:"appendTolerations,omitempty"`
	// OverrideAffinity is an affinity that will be used to override the agent deployment's affinity.
	// +optional
	OverrideAffinity *v1.Affinity `json:"overrideAffinity,omitempty"`
	// OverrideResourceRequirements defines the limits, requests, and claims of compute resources for a given container.
	// +optional
	OverrideResourceRequirements *v1.ResourceRequirements `json:"overrideResourceRequirements,omitempty"`
	// SchedulingCustomization is an optional configuration that will be used to override the agent deployment's scheduling customization.
	// +optional
	SchedulingCustomization *AgentSchedulingCustomization `json:"schedulingCustomization,omitempty"`
}

type AgentSchedulingCustomization struct {
	// +optional
	PriorityClass *PriorityClassSpec `json:"priorityClass,omitempty"`
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
}

type PriorityClassSpec struct {
	// +optional
	Value int `json:"value,omitempty"`
	// +optional
	PreemptionPolicy *v1.PreemptionPolicy `json:"preemptionPolicy,omitempty"`
}

type PodDisruptionBudgetSpec struct {
	// +optional
	MinAvailable string `json:"minAvailable,omitempty"`
	// +optional
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
}

type ClusterStatus struct {
	// Ready reflects whether the cluster's ready state has previously been reported as true.
	// +optional
	Ready bool `json:"ready,omitempty"`
	// Name of the cluster.management.cattle.io object that relates to this cluster.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// FleetWorkspaceName is the name of the fleet workspace that the cluster belongs to.
	// Defaults to the namespace of the cluster object, which is usually "fleet-default".
	// +optional
	FleetWorkspaceName string `json:"fleetWorkspaceName,omitempty"`
	// ClientSecretName is the name of the kubeconfig secret that is used to connect to the cluster.
	// This secret is typically named "<cluster-name>-kubeconfig" and lives in the namespace of the cluster object.
	// +optional
	ClientSecretName string `json:"clientSecretName,omitempty"`
	// AgentDeployed reflects whether the cluster agent has been deployed successfully.
	// +optional
	AgentDeployed bool `json:"agentDeployed,omitempty"`
	// ObservedGeneration is the most recent generation for which the management cluster object was generated for the corresponding provisioning cluster spec.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration"`
	// Conditions is a representation of the Cluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type ImportedConfig struct {
	KubeConfigSecretName string `json:"kubeConfigSecretName,omitempty"`
}

type ClusterAPIConfig struct {
	ClusterName string `json:"clusterName,omitempty"`
}
