package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateRotationArgs contains parameters for rotating certificates.
type CertificateRotationArgs struct {
	// Services is a list of services to rotate certificates for.
	// If empty, all supported services are rotated.
	// +kubebuilder:validation:items:Enum=admin;api-server;auth-proxy;cloud-controller;controller-manager;etcd;k3s-controller;k3s-server;kubelet;kube-proxy;rke2-controller;rke2-server;scheduler
	// +nullable
	// +optional
	Services []string `json:"services,omitempty"`
}

// CertificateRotationSpec defines the desired state of CertificateRotation.
type CertificateRotationSpec struct {
	// OperationSpec is the shared spec common to all operations.
	OperationSpec `json:",inline"`

	// Args contains parameters for certificate rotation.
	// +optional
	Args CertificateRotationArgs `json:"args,omitempty"`
}

// CertificateRotationStep is the step of the CertificateRotation operation.
type CertificateRotationStep string

const (
	// CertificateRotationStepRotate indicates the step is rotating certificates.
	CertificateRotationStepRotate CertificateRotationStep = "Rotate"
)

// CertificateRotationStatus defines the observed state of CertificateRotation.
type CertificateRotationStatus struct {
	// OperationStatus is the shared status common to all operations.
	OperationStatus `json:",inline"`

	// Step is the current step of the operation.
	// Step is typically only valid during the InProgress phase.
	// +kubebuilder:validation:Enum=Rotate
	// +optional
	Step CertificateRotationStep `json:"step,omitempty"`
}

func (s *CertificateRotationStatus) SetPhase(phase OperationPhase) {
	if s.Phase == phase {
		return
	}
	s.Phase = phase
	s.LastUpdated = metav1.Now()
}

func (s *CertificateRotationStatus) SetStep(step CertificateRotationStep) {
	if s.Step == step {
		return
	}
	s.Step = step
	s.LastUpdated = metav1.Now()
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=certificaterotations,scope=Namespaced,categories=operations
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="Services",type=string,JSONPath=".spec.args.services"
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=".spec.paused"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Step",type=string,JSONPath=".status.step"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// CertificateRotation is the mechanism for initiating an RKE2 or K3s certificate rotation
// operation for provisioned and imported clusters.
type CertificateRotation struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the CertificateRotation.
	// +required
	Spec CertificateRotationSpec `json:"spec,omitempty"`

	// Status is the observed state of the CertificateRotation.
	// +optional
	Status CertificateRotationStatus `json:"status,omitempty"`
}
