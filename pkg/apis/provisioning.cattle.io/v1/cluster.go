package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterSpec   `json:"spec"`
	Status            ClusterStatus `json:"status,omitempty"`
}

type ClusterSpec struct {
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`
	KubernetesVersion         string `json:"kubernetesVersion,omitempty"`

	ClusterAPIConfig         *ClusterAPIConfig              `json:"clusterAPIConfig,omitempty"`
	RKEConfig                *RKEConfig                     `json:"rkeConfig,omitempty"`
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`

	AgentEnvVars                                         []rkev1.EnvVar                `json:"agentEnvVars,omitempty"`
	ClusterAgentDeploymentCustomization                  *AgentDeploymentCustomization `json:"clusterAgentDeploymentCustomization,omitempty"`
	DefaultPodSecurityAdmissionConfigurationTemplateName string                        `json:"defaultPodSecurityAdmissionConfigurationTemplateName,omitempty"`
	DefaultClusterRoleForProjectMembers                  string                        `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	EnableNetworkPolicy                                  *bool                         `json:"enableNetworkPolicy,omitempty" norman:"default=false"`
	FleetAgentDeploymentCustomization                    *AgentDeploymentCustomization `json:"fleetAgentDeploymentCustomization,omitempty"`

	RedeploySystemAgentGeneration int64 `json:"redeploySystemAgentGeneration,omitempty"`
}

type AgentDeploymentCustomization struct {
	AppendTolerations            []v1.Toleration          `json:"appendTolerations,omitempty"`
	OverrideAffinity             *v1.Affinity             `json:"overrideAffinity,omitempty"`
	OverrideResourceRequirements *v1.ResourceRequirements `json:"overrideResourceRequirements,omitempty"`
}

type ClusterStatus struct {
	Ready              bool                                `json:"ready,omitempty"`
	ClusterName        string                              `json:"clusterName,omitempty"`
	FleetWorkspaceName string                              `json:"fleetWorkspaceName,omitempty"`
	ClientSecretName   string                              `json:"clientSecretName,omitempty"`
	AgentDeployed      bool                                `json:"agentDeployed,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`
	Conditions         []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type ImportedConfig struct {
	KubeConfigSecretName string `json:"kubeConfigSecretName,omitempty"`
}

type ClusterAPIConfig struct {
	ClusterName string `json:"clusterName,omitempty"`
}
