// +k8s:deepcopy-gen=package
// +groupName=management.cattle.io
package v3

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster

type OIDCClient struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired configuration for the oidc client.
	// +optional
	Spec OIDCClientSpec `json:"spec,omitempty"`

	// Status is the most recently observed status of the oidc client.
	// +optional
	Status OIDCClientStatus `json:"status,omitempty"`
}

// OIDCClientStatus represents the most recently observed status of the oidc client.
type OIDCClientStatus struct {
	ClientID string `json:"clientID,omitempty"`
}

// OIDCClient is a description of the oidc client.
type OIDCClientSpec struct {
	// +optional
	Description string `json:"description,omitempty"`
	// +optional
	RedirectURIs []string `json:"redirectURIs"`
	// +optional
	TokenExpirationSeconds time.Duration `json:"tokenExpirationSeconds"`
	// +optional
	RefreshTokenExpirationSeconds time.Duration `json:"refreshTokenExpirationSeconds"`
}
