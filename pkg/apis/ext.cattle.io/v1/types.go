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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kubeconfig contains information about clusters, users, namespaces, and authentication mechanisms.
type Kubeconfig struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the desired state of the kubeconfig.
	// +optional
	Spec KubeconfigSpec `json:"spec,omitempty"`
	// Status is the most recently observed status of the kubeconfig.
	// +optional
	Status *KubeconfigStatus `json:"status,omitempty"`
}

// KubeconfigSpec defines the desired state of Kubeconfig.
type KubeconfigSpec struct {
	// Clusters is a list of cluster names.
	// +listType=set
	// +optional
	Clusters []string `json:"clusters"`
	// CurrentContext is the cluster ID default context for which will be set as the current context.
	// If omitted, the first cluster in the list is considered for setting the current context.
	// +optional
	CurrentContext string `json:"currentContext,omitempty"`
}

// KubeconfigStatus defines the most recently observed status of the kubeconfig.
type KubeconfigStatus struct {
	// Value contains the generated kubeconfig.
	Value string `json:"value,omitempty"`
}
