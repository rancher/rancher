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
	// TokenId is the token Id for which the UserActivity will update
	// the LastIdleTimeout value on the Token resource.
	// +optional
	TokenId string `json:"tokenId"`
}

type UserActivityStatus struct {
	// CurrentTimeout is the timestap of the idle timeout.
	// The idle timeout is calculated by adding the
	// auth-user-session-idle-ttl-minutes attribute to the time
	// the request is made.
	// +optional
	CurrentTimeout string `json:"currentTimeout"`
	// LastActivity is the timestamp of the last user activity
	// tracked by the UI.
	// +optional
	LastActivity string `json:"lastActivity,omitempty"`
}
