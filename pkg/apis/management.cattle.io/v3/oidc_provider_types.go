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
// +kubebuilder:printcolumn:name="Client ID",type="string",JSONPath=".status.clientID"
// +kubebuilder:printcolumn:name="Token Expiration",type="integer",JSONPath=".spec.tokenExpirationSeconds"
// +kubebuilder:printcolumn:name="RefreshToken Expiration",type="integer",JSONPath=".spec.refreshTokenExpirationSeconds"
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
	// Description provides additional context about the OIDC client.
	// +optional
	Description string `json:"description,omitempty"`
	// RedirectURIs defines the allowed redirect URIs for the OIDC client.
	// These URIs must be registered and used during the authentication flow.
	// +optional
	RedirectURIs []string `json:"redirectURIs"`
	// TokenExpirationSeconds specifies the duration (in seconds) before
	// an access token and ID token expire.
	TokenExpirationSeconds time.Duration `json:"tokenExpirationSeconds"`
	// RefreshTokenExpirationSeconds defines how long (in seconds)
	// a refresh token remains valid before expiration.
	RefreshTokenExpirationSeconds time.Duration `json:"refreshTokenExpirationSeconds"`
}
