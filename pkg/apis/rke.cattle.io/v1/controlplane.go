package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKEControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEControlPlaneSpec   `json:"spec"`
	Status            RKEControlPlaneStatus `json:"status,omitempty"`
}

type RKEControlPlaneSpec struct {
	RKEClusterSpecCommon

	KubernetesVersion     string `json:"kubernetesVersion,omitempty"`
	ManagementClusterName string `json:"managementClusterName,omitempty" wrangler:"required"`
}

type RKEControlPlaneStatus struct {
	Conditions             []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready                  bool                                `json:"ready,omitempty"`
	ObservedGeneration     int64                               `json:"observedGeneration"`
	ClusterStateSecretName string                              `json:"clusterStateSecretName,omitempty"`
}
