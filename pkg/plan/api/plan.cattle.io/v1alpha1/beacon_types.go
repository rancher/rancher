package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type BeaconSpec struct {
}

type BeaconStatus struct {
	// Conditions is a representation of the Cluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Active denotes whether the cluster is currently running a plan.
	// System-agents should download connection information when the beacon is active.
	Active bool `json:"active"`

	// RegistrationEndpoint is the URL a system-agent must query to download its machine-plan and corresponding
	// kubeconfig.
	RegistrationEndpoint string `json:"registrationEndpoint,omitempty"`
}

// +genclient
// +kubebuilder:resource:path=beacons,scope=Namespaced,categories=planning
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels={"auth.cattle.io/cluster-indexed=true"}
// +kubebuilder:printcolumn:name="Active",type=string,JSONPath=".status.active"
// +kubebuilder:printcolumn:name="Registration Endpoint",type=string,JSONPath=".status.registrationEndpoint"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// The Beacon is a record indicating whether the system-agents should expect to receive plans.
type Beacon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BeaconSpec   `json:"spec,omitempty"`
	Status BeaconStatus `json:"status,omitempty"`
}
