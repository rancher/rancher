package v1

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
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

	ClusterAPIConfig         *ClusterAPIConfig           `json:"clusterAPIConfig,omitempty"`
	RKEConfig                *RKEConfig                  `json:"rkeConfig,omitempty"`
	ReferencedConfig         *ReferencedConfig           `json:"referencedConfig,omitempty"`
	RancherValues            v1.GenericMap               `json:"rancherValues,omitempty" wrangler:"nullable"`
	LocalClusterAuthEndpoint v3.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty"`
}

type ClusterStatus struct {
	Ready              bool                                `json:"ready,omitempty"`
	ClusterName        string                              `json:"clusterName,omitempty"`
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

type ReferencedConfig struct {
	ManagementClusterName string `json:"managementClusterName,omitempty"`
}
