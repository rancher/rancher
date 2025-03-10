// +kubebuilder:skip
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserActivity keeps tracks user activity in the UI.
// If the user doesn't perform certain actions for a while e.g. cursor moved, key pressed, etc.,
// this will lead to the user being logged out of the session.
type UserActivity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status is the most recently observed status of the UserActivity.
	Status UserActivityStatus `json:"status"`
}

// UserActivityStatus defines the most recently observed status of the UserActivity.
type UserActivityStatus struct {
	// ExpiresAt is the timestamp at which the user's session expires if it stays idle, invalidating the corresponding session token.
	// It is calculated by adding the duration specified in the auth-user-session-idle-ttl-minutes setting to the time of the request.
	// +optional
	ExpiresAt string `json:"expiresAt"`
}
