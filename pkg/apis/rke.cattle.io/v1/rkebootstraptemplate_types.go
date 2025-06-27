package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RKEBootstrapTemplateSpec defines the desired state of RKEBootstrapTemplate.
type RKEBootstrapTemplateSpec struct {
	// ClusterName refers to the name of the CAPI Cluster associated with this RKEBootstrap.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Template defines the desired state of RKEBootstrapTemplateSpec.
	// +required
	Template RKEBootstrap `json:"template"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=rkebootstraptemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:metadata:labels={"cluster.x-k8s.io/v1beta1=v1","auth.cattle.io/cluster-indexed=true"}

// RKEBootstrapTemplate is the schema for the rkebootstraptemplates API.
type RKEBootstrapTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of RKEBootstrapTemplate.
	// +required
	Spec RKEBootstrapTemplateSpec `json:"spec"`
}
