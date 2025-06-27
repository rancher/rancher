package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RKECommonNodeConfig struct {
	// +optional
	// +nullable
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	// +nullable
	Taints []corev1.Taint `json:"taints,omitempty"`

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
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CustomMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomMachineSpec   `json:"spec,omitempty"`
	Status CustomMachineStatus `json:"status,omitempty"`
}

type CustomMachineSpec struct {
	ProviderID string `json:"providerID,omitempty"`
}

type CustomMachineStatus struct {
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready      bool                                `json:"ready,omitempty"`
	Addresses  []capi.MachineAddress               `json:"addresses,omitempty"`
}
