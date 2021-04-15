package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RKECommonNodeConfig struct {
	HostnamePrefix            string            `json:"hostnamePrefix,omitempty"`
	Labels                    map[string]string `json:"labels,omitempty"`
	Taints                    []corev1.Taint    `json:"taints,omitempty"`
	CloudCredentialSecretName string            `json:"cloudCredentialSecretName,omitempty"`
}

type RKEMachineStatus struct {
	JobComplete               bool   `json:"jobComplete,omitempty"`
	Ready                     bool   `json:"ready,omitempty"`
	DriverHash                string `json:"driverHash,omitempty"`
	DriverURL                 string `json:"driverUrl,omitempty"`
	CloudCredentialSecretName string `json:"cloudCredentialSecretName,omitempty"`
	FailureReason             string `json:"failureReason,omitempty"`
	FailureMessage            string `json:"failureMessage,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type UnmanagedMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnmanagedMachineSpec   `json:"spec,omitempty"`
	Status UnmanagedMachineStatus `json:"status,omitempty"`
}

type UnmanagedMachineSpec struct {
}

type UnmanagedMachineStatus struct {
	Ready bool `json:"ready,omitempty"`
}
