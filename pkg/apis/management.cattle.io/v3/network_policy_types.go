package v3

import (
	"strings"

	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectNetworkPolicySpec struct {
	ProjectName string `json:"projectName,omitempty" norman:"required,type=reference[project]"`
	Description string `json:"description"`
}

func (p *ProjectNetworkPolicySpec) ObjClusterName() string {
	if parts := strings.SplitN(p.ProjectName, ":", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type ProjectNetworkPolicyStatus struct {
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProjectNetworkPolicy struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ProjectNetworkPolicySpec    `json:"spec"`
	Status            *ProjectNetworkPolicyStatus `json:"status"`
}
