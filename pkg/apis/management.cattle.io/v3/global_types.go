package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value      string `json:"value" norman:"required"`
	Default    string `json:"default" norman:"nocreate,noupdate"`
	Customized bool   `json:"customized" norman:"nocreate,noupdate"`
	Source     string `json:"source" norman:"nocreate,noupdate,options=db|default|env"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Feature struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureSpec   `json:"spec"`
	Status FeatureStatus `json:"status"`
}

type FeatureSpec struct {
	Value *bool `json:"value" norman:"required"`
}

type FeatureStatus struct {
	Dynamic     bool   `json:"dynamic"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
	LockedValue *bool  `json:"lockedValue"`
}

const (
	ExperimentalFeatureKey   = "feature.cattle.io/experimental"
	ExperimentalFeatureValue = "true"
)
