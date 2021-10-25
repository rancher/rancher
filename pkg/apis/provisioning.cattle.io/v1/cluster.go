package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
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

	AgentEnvVars                         []rkev1.EnvVar `json:"agentEnvVars,omitempty"`
	DefaultPodSecurityPolicyTemplateName string         `json:"defaultPodSecurityPolicyTemplateName,omitempty" norman:"type=reference[podSecurityPolicyTemplate]"`
	DefaultClusterRoleForProjectMembers  string         `json:"defaultClusterRoleForProjectMembers,omitempty" norman:"type=reference[roleTemplate]"`
	EnableNetworkPolicy                  *bool          `json:"enableNetworkPolicy,omitempty" norman:"default=false"`

	RedeploySystemAgentGeneration int64 `json:"redeploySystemAgentGeneration,omitempty"`
}

type ClusterStatus struct {
	Ready              bool                                `json:"ready,omitempty"`
	ClusterName        string                              `json:"clusterName,omitempty"`
	ClientSecretName   string                              `json:"clientSecretName,omitempty"`
	AgentDeployed      bool                                `json:"agentDeployed,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`
	Conditions         []genericcondition.GenericCondition `json:"conditions,omitempty"`
	ETCDSnapshots      []rkev1.ETCDSnapshot                `json:"etcdSnapshots,omitempty"`
}

type ImportedConfig struct {
	KubeConfigSecretName string `json:"kubeConfigSecretName,omitempty"`
}

type ClusterAPIConfig struct {
	ClusterName string `json:"clusterName,omitempty"`
}
