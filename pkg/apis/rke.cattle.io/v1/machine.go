package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type RKECommonNodeConfig struct {
	//+optional
	//+nullable
	Labels map[string]string `json:"labels,omitempty"`

	//+optional
	//+nullable
	Taints []corev1.Taint `json:"taints,omitempty"`

	//+optional
	//+nullable
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`
}

type RKEMachineStatus struct {
	//+optional
	//+nullable
	// Conditions is a representation of the machine's current state.
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	//+optional
	//+nullable
	// JobName is the name of the provisioning job of the machine.
	JobName string `json:"jobName,omitempty"`

	// Ready indicates whether the provider ID has been set in this machine's spec.
	Ready bool `json:"ready,omitempty"`

	//+optional
	//+nullable
	// DriverHash is the expected hash of the node driver binary used for provisioning the machine.
	DriverHash string `json:"driverHash,omitempty"`

	//+optional
	//+nullable
	// DriverURL is the url used to download the node driver binary for provisioning the machine.
	DriverURL string `json:"driverUrl,omitempty"`

	//+optional
	//+nullable
	// CloudCredentialSecretName is the secret name that was used as a credential to provision the machine.
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`

	//+optional
	//+nullable
	// FailureReason indicates whether the provisioning job failed on creation or on removal of the machine.
	FailureReason string `json:"failureReason,omitempty"`

	//+optional
	//+nullable
	// FailureMessage is the container termination message for a provisioning job that failed.
	FailureMessage string `json:"failureMessage,omitempty"`

	//+optional
	//+nullable
	// Addresses are the machine network addresses. Assigned by the CAPI controller.
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
