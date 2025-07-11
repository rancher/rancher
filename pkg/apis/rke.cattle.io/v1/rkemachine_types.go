package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// RKECommonNodeConfig is a common configuration shared between node driver and custom machines.
type RKECommonNodeConfig struct {
	// Labels is a list of labels to apply to the machines created by the CAPI machine deployment.
	// +optional
	// +nullable
	Labels map[string]string `json:"labels,omitempty"`
	// Taints is a list of taints to apply to the machines created by the CAPI machine deployment.
	// +optional
	// +nullable
	Taints []corev1.Taint `json:"taints,omitempty"`
	// CloudCredentialSecretName is the id of the secret used to provision
	// the cluster.
	// This field must be in the format of "namespace:name".
	// NOTE: this field overrides the field of the same name on the cluster
	// spec, allowing individual machine pools to use separate credentials.
	// +kubebuilder:validation:MaxLength=317
	// +optional
	// +nullable
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`
}

type RKEMachineStatus struct {
	// Conditions is a representation of the machine's current state.
	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// JobName is the name of the provisioning job of the machine.
	// +optional
	JobName string `json:"jobName,omitempty"`

	// Ready indicates whether the provider ID has been set in this machine's spec.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// DriverHash is the expected hash of the node driver binary used for provisioning the machine.
	// +optional
	DriverHash string `json:"driverHash,omitempty"`

	// DriverURL is the url used to download the node driver binary for provisioning the machine.
	// +optional
	DriverURL string `json:"driverUrl,omitempty"`

	// CloudCredentialSecretName is the secret name that was used as a credential to provision the machine.
	// +optional
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`

	// FailureReason indicates whether the provisioning job failed on creation or on removal of the machine.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// FailureMessage is the container termination message for a provisioning job that failed.
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// Addresses are the machine network addresses. Assigned by the CAPI controller.
	// +optional
	Addresses []capi.MachineAddress `json:"addresses,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"cluster.x-k8s.io/v1beta1=v1","auth.cattle.io/cluster-indexed=true"}

// CustomMachine represents an unmanaged CAPI
// machine registered to a Rancher custom cluster.
type CustomMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired configuration of the machine.
	Spec CustomMachineSpec `json:"spec,omitempty"`

	// Status represents the most recently observed status of the machine.
	// +optional
	Status CustomMachineStatus `json:"status,omitempty"`
}

type CustomMachineSpec struct {
	// ProviderID is a reference to the CAPI Node object corresponding to
	// this machine. This field is automatically set by CAPI during the
	// machine provisioning process.
	// +optional
	// +nullable
	ProviderID string `json:"providerID,omitempty"`
}

type CustomMachineStatus struct {
	// Conditions is a representation of the current state of the machine.
	// +optional
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
	// Ready indicates that the machine infrastructure is fully provisioned,
	// and is a requirement of the Cluster API contract. The value of this
	// field is never updated after provisioning has completed.
	// Please use Conditions to determine the current state of the machine.
	// +optional
	Ready bool `json:"ready,omitempty"`
	// Addresses contains the associated addresses for the machine.
	// +optional
	Addresses []capi.MachineAddress `json:"addresses,omitempty"`
}
