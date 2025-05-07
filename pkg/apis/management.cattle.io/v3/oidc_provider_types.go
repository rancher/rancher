// +k8s:deepcopy-gen=package
// +groupName=management.cattle.io
package v3

import (
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

// OIDCClientSecretStatus represent the observed status of a client secret.
type OIDCClientSecretStatus struct {
	// LastUsedAt represent when this client secret was used.
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	// CreatedAt represents when this client secret was created.
	CreatedAt string `json:"createdAt,omitempty"`
	// LastFiveCharacters are the 5 last characters of the client secret
	LastFiveCharacters string `json:"lastFiveCharacters,omitempty"`
}

// OIDCClientStatus represents the most recently observed status of the oidc client.
type OIDCClientStatus struct {
	// ClientID represents the ID of the client
	ClientID string `json:"clientID,omitempty"`
	// ClientSecrets represents the observed status of the client secrets
	ClientSecrets map[string]OIDCClientSecretStatus `json:"clientSecrets,omitempty"`
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
	// +kubebuilder:validation:Minimum=1
	TokenExpirationSeconds int64 `json:"tokenExpirationSeconds"`
	// RefreshTokenExpirationSeconds defines how long (in seconds)
	// a refresh token remains valid before expiration.
	// +kubebuilder:validation:Minimum=1
	RefreshTokenExpirationSeconds int64 `json:"refreshTokenExpirationSeconds"`
}
