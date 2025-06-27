package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RKEClusterSpec struct {
	// Not used in anyway, just here to make cluster-api happy
	ControlPlaneEndpoint *capi.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

type RKEClusterStatus struct {
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready      bool                                `json:"ready,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEClusterSpec   `json:"spec"`
	Status            RKEClusterStatus `json:"status,omitempty"`
}
