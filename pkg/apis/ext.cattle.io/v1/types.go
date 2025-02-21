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

	// Status contains system information about the useractivity.
	Status UserActivityStatus `json:"status"`
}

type UserActivityStatus struct {
	// CurrentTimeout is the timestamp at which the idle timer expires, invalidating the Token and session.
	// It is calculated by adding the
	// auth-user-session-idle-ttl-minutes attribute to the time
	// the request is made.
	// +optional
	ExpiresAt string `json:"expiresAt"`
}
