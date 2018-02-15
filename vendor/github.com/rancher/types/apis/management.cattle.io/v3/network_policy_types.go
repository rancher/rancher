package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectNetworkPolicySpec struct {
	ProjectName string `json:"projectName,omitempty" norman:"required,type=reference[project]"`
	Description string `json:"description"`
}

type ProjectNetworkPolicyStatus struct {
}

type ProjectNetworkPolicy struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectNetworkPolicySpec    `json:"spec"`
	Status            *ProjectNetworkPolicyStatus `json:"status"`
}
