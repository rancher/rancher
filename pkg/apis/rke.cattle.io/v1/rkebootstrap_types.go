package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RKEBootstrapSpec defines the desired state of RKEBootstrap.
type RKEBootstrapSpec struct {
	// ClusterName refers to the name of the CAPI Cluster associated with this RKEBootstrap.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}

// RKEBootstrapStatus defines the observed state of RKEBootstrap.
type RKEBootstrapStatus struct {
	// Ready indicates the BootstrapData field is ready to be consumed.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	DataSecretName *string `json:"dataSecretName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=rkebootstraps,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"cluster.x-k8s.io/v1beta1=v1","auth.cattle.io/cluster-indexed=true"}

// RKEBootstrap defines the BootstrapConfig resource required by Cluster API
// to supply the bootstrap data necessary for initializing a Kubernetes node.
// It is referenced by one of the core Cluster API resources, Machine.
// More info: https://cluster-api.sigs.k8s.io/developer/providers/contracts/bootstrap-config
type RKEBootstrap struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of RKEBootstrap.
	Spec RKEBootstrapSpec `json:"spec"`

	// Status is the observed state of RKEBootstrap.
	// +optional
	Status RKEBootstrapStatus `json:"status,omitempty"`
}
