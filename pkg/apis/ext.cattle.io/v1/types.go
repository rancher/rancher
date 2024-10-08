// +kubebuilder:skip
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// User Activity
type UserActivity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the spec of UserActivity
	Spec UserActivitySpec `json:"spec"`
	// +optional
	Status UserActivityStatus `json:"status"`
}

type UserActivitySpec struct {
	TokenId string `json:"tokenId"`
}

type UserActivityStatus struct {
	CurrentTimeout string `json:"currentTimeout"`
	LastActivity   string `json:"lastActivity"`
}
