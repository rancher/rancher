package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EncryptionKeyRotationSpec defines the desired state of EncryptionKeyRotation.
type EncryptionKeyRotationSpec struct {
	// OperationSpec contains the shared operation inputs, including the required ClusterRef.
	OperationSpec `json:",inline"`
}

// EncryptionKeyRotationStep is the step of the EncryptionKeyRotation operation.
type EncryptionKeyRotationStep string

const (
	// EncryptionKeyRotationStepRotate indicates the step is to rotate the encryption keys
	// by running the secrets-encrypt rotate-keys command on the elected control-plane leader.
	EncryptionKeyRotationStepRotate EncryptionKeyRotationStep = "Rotate"

	// EncryptionKeyRotationStepRestart indicates the step is to restart the distro server
	// service on all control plane nodes after key rotation.
	EncryptionKeyRotationStepRestart EncryptionKeyRotationStep = "Restart"
)

// EncryptionKeyRotationStatus defines the observed state of EncryptionKeyRotation.
type EncryptionKeyRotationStatus struct {
	// OperationStatus is the shared status common to all operations.
	OperationStatus `json:",inline"`

	// Step is the current step of the operation.
	// Step is typically only valid during the InProgress phase.
	// +kubebuilder:validation:Enum=Rotate;Restart
	// +optional
	Step EncryptionKeyRotationStep `json:"step,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=encryptionkeyrotations,scope=Namespaced,categories=operations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=".spec.paused"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=".status.step"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// EncryptionKeyRotation is the mechanism for initiating an encryption key rotation
// operation for provisioned or imported RKE2/K3s clusters.
type EncryptionKeyRotation struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EncryptionKeyRotation.
	// +required
	Spec EncryptionKeyRotationSpec `json:"spec,omitempty"`

	// Status is the observed state of the EncryptionKeyRotation.
	// +optional
	Status EncryptionKeyRotationStatus `json:"status,omitempty"`
}
