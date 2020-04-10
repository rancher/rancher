/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ANCHOR: MachineHealthCheckSpec

// MachineHealthCheckSpec defines the desired state of MachineHealthCheck
type MachineHealthCheckSpec struct {
	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// Label selector to match machines whose health will be exercised
	Selector metav1.LabelSelector `json:"selector"`

	// UnhealthyConditions contains a list of the conditions that determine
	// whether a node is considered unhealthy.  The conditions are combined in a
	// logical OR, i.e. if any of the conditions is met, the node is unhealthy.
	//
	// +kubebuilder:validation:MinItems=1
	UnhealthyConditions []UnhealthyCondition `json:"unhealthyConditions"`

	// Any further remediation is only allowed if at most "MaxUnhealthy" machines selected by
	// "selector" are not healthy.
	// +optional
	MaxUnhealthy *intstr.IntOrString `json:"maxUnhealthy,omitempty"`

	// Machines older than this duration without a node will be considered to have
	// failed and will be remediated.
	// +optional
	NodeStartupTimeout *metav1.Duration `json:"nodeStartupTimeout,omitempty"`
}

// ANCHOR_END: MachineHealthCHeckSpec

// ANCHOR: UnhealthyCondition

// UnhealthyCondition represents a Node condition type and value with a timeout
// specified as a duration.  When the named condition has been in the given
// status for at least the timeout value, a node is considered unhealthy.
type UnhealthyCondition struct {
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	Type corev1.NodeConditionType `json:"type"`

	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MinLength=1
	Status corev1.ConditionStatus `json:"status"`

	Timeout metav1.Duration `json:"timeout"`
}

// ANCHOR_END: UnhealthyCondition

// ANCHOR: MachineHealthCheckStatus

// MachineHealthCheckStatus defines the observed state of MachineHealthCheck
type MachineHealthCheckStatus struct {
	// total number of machines counted by this machine health check
	// +kubebuilder:validation:Minimum=0
	ExpectedMachines int32 `json:"expectedMachines,omitempty"`

	// total number of healthy machines counted by this machine health check
	// +kubebuilder:validation:Minimum=0
	CurrentHealthy int32 `json:"currentHealthy,omitempty"`
}

// ANCHOR_END: MachineHealthCheckStatus

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machinehealthchecks,shortName=mhc;mhcs,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="MaxUnhealthy",type="string",JSONPath=".spec.maxUnhealthy",description="Maximum number of unhealthy machines allowed"
// +kubebuilder:printcolumn:name="ExpectedMachines",type="integer",JSONPath=".status.expectedMachines",description="Number of machines currently monitored"
// +kubebuilder:printcolumn:name="CurrentHealthy",type="integer",JSONPath=".status.currentHealthy",description="Current observed healthy machines"

// MachineHealthCheck is the Schema for the machinehealthchecks API
type MachineHealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of machine health check policy
	Spec MachineHealthCheckSpec `json:"spec,omitempty"`

	// Most recently observed status of MachineHealthCheck resource
	Status MachineHealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MachineHealthCheckList contains a list of MachineHealthCheck
type MachineHealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineHealthCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MachineHealthCheck{}, &MachineHealthCheckList{})
}
