package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NavLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NavLinkSpec `json:"spec"`
}

type NavLinkSpec struct {
	Label       string                `json:"label,omitempty"`
	Description string                `json:"description,omitempty"`
	SideLabel   string                `json:"sideLabel,omitempty"`
	IconSrc     string                `json:"iconSrc,omitempty"`
	Group       string                `json:"group,omitempty"`
	Target      string                `json:"target,omitempty"`
	ToURL       string                `json:"toURL,omitempty"`
	ToService   *NavLinkTargetService `json:"toService,omitempty"`
}

type NavLinkTargetService struct {
	Namespace string              `json:"namespace,omitempty" wrangler:"required"`
	Name      string              `json:"name,omitempty" wrangler:"required"`
	Scheme    string              `json:"scheme,omitempty" wrangler:"default=http,options=http|https,type=enum"`
	Port      *intstr.IntOrString `json:"port,omitempty"`
	Path      string              `json:"path,omitempty"`
}
