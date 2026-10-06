package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateRotationArgs contains parameters for rotating certificates.
type CertificateRotationArgs struct {
	// Services is a list of services to rotate certificates for.
	// If empty, all supported services are rotated.
	// +kubebuilder:validation:items:Enum=admin;api-server;auth-proxy;cloud-controller;controller-manager;etcd;k3s-controller;k3s-server;kubelet;kube-proxy;rke2-controller;rke2-server;scheduler;supervisor
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
	// CertificateRotationStepPreflight indicates the step is checking that the rotation can proceed
	// (that the cluster has nodes to rotate, and the services requested exist on them) before anything
	// on the cluster is changed. The cluster is paused as the operation leaves this step, which is its
	// point of no return: stopped before it, the rotation leaves nothing to repair.
	CertificateRotationStepPreflight CertificateRotationStep = "Preflight"

	// CertificateRotationStepRotate indicates the step is rotating certificates.
	CertificateRotationStepRotate CertificateRotationStep = "Rotate"
)

// CertificateRotationStatus defines the observed state of CertificateRotation.
type CertificateRotationStatus struct {
	// OperationStatus is the shared status common to all operations.
	OperationStatus `json:",inline"`

	// Step is the current step of the operation.
	// Step is typically only valid during the InProgress phase.
	// +kubebuilder:validation:Enum=Preflight;Rotate
	// +optional
	Step CertificateRotationStep `json:"step,omitempty"`
}

// SetStep sets the status step, updating LastUpdated only when the value changes.
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
// +kubebuilder:validation:XValidation:rule="!self.spec.cancel || oldSelf.spec.cancel || !has(self.status) || !has(self.status.phase) || !(self.status.phase in ['Succeeded','Failed','Rejected','Canceled'])",message="cancel cannot be set once the operation has reached a terminal phase; delete the operation instead"
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
