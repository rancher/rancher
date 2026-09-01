package v3

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProxyEndpoint defines a set of domains to be added to the Rancher meta proxy allowlist,
// which determines what external domains the proxy is permitted to forward requests to.
// Domain entries support absolute domain names (e.g., example.com) and wildcard
// patterns using * (prefix matching) or % (single-segment placeholder).
type ProxyEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProxyEndpointSpec   `json:"spec,omitempty"`
	Status ProxyEndpointStatus `json:"status,omitempty"`
}

type ProxyEndpointSpec struct {
	// Routes is a list of domains that will be added to the meta proxy
	// allowlist. These are expected to be domain names, and not URL paths.
	// +required
	Routes []ProxyEndpointRoute `json:"routes,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!(has(self.caBundle) && self.caBundle != ” && self.insecureSkipTLSVerify)",message="caBundle cannot be set when insecureSkipTLSVerify is true"
type ProxyEndpointRoute struct {
	// Domain is the domain to be added to the proxy allowlist.
	// Absolute domain names (e.g., example.com) and wildcard patterns are supported.
	// There are two types of supported wildcard patterns:
	//
	// 1) Prefix wildcard (*): Matches any subdomain or prefix. Can only appear as the
	//    leftmost character of the domain. For example:
	//    - *.example.com matches foo.example.com and bar.example.com
	//    - *test.com matches footest.com and contest.com
	//    The wildcard character is taken literally; *.*.com is not valid.
	//
	// 2) Single-segment placeholder (%): Matches exactly one domain segment.
	//    Can be used as the leftmost segment or within the domain as a complete label.
	//    For example:
	//    - %.example.com matches foo.example.com but not foo.bar.example.com
	//    - ec2.%.aws.com matches ec2.us-east-1.aws.com but not ec2.us.east.aws.com
	//    The placeholder must be a complete label; %test.com is not valid.
	//
	// Both types of wildcards may be combined (e.g., *.%.example.com).
	// However, overly broad wildcard patterns that match a large number of
	// domains (e.g., "*", "%", "*.*", "*.com", "%.com", "*.co.uk") are not allowed.
	// A webhook validates that patterns include sufficient concrete domain content.
	//
	//
	// Domain entries should not include URL schemes (e.g., "https://").
	// For example, "example.com" is valid, but "https://example.com" is not.
	// It is assumed that all provided domains use HTTPS, and the proxy will route accordingly.
	//
	// +required
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^(\*\.?)?([a-zA-Z0-9][-a-zA-Z0-9]{0,61}[a-zA-Z0-9]?|(%\.([a-zA-Z0-9][-a-zA-Z0-9]{0,61}[a-zA-Z0-9]?|%))+)(\.(([a-zA-Z0-9][-a-zA-Z0-9]{0,61}[a-zA-Z0-9]?)|%))*\.[a-zA-Z][-a-zA-Z0-9]{0,61}[a-zA-Z0-9]$`
	Domain string `json:"domain,omitempty"`

	// InsecureSkipTLSVerify disables TLS certificate verification when proxying to this domain.
	// Use this only for development or when the endpoint uses a self-signed certificate.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`

	// CABundle is a PEM-encoded bundle of CA certificates to trust when verifying the TLS
	// certificate of the endpoint. This is useful for self-signed certificates or certificates
	// from non-public Certificate Authorities.
	// When specified, only these CA certificates (plus the system's root CAs) will be trusted
	// for verifying the endpoint's certificate. Do not use this with InsecureSkipTLSVerify.
	// +optional
	// +kubebuilder:validation:MaxLength=100000
	CABundle string `json:"caBundle,omitempty"`

	// ClientCertificate references a Kubernetes Secret containing client certificate and key
	// for mutual TLS (mTLS) authentication. The secret must be in the same namespace as the
	// ProxyEndpoint. The secret must contain "tls.crt" and "tls.key" fields with PEM-encoded
	// certificate and private key respectively.
	// +optional
	ClientCertificate *SecretReference `json:"clientCertificate,omitempty"`

	// ServerName is the SNI (Server Name Indication) hostname to use during the TLS handshake.
	// If not specified, the domain hostname is used. This is useful when the endpoint domain
	// does not match the certificate's CN or SAN fields.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	ServerName string `json:"serverName,omitempty"`

	// TLSVerificationOptions controls certificate verification behavior.
	// +optional
	TLSVerificationOptions *TLSVerificationSpec `json:"tlsVerificationOptions,omitempty"`

	// CredentialInjection defines how credentials are applied to proxied requests for this domain.
	// When set, clients need to supply a credential ID and values for the secret fields via X-API-CattleAuth-Header;

	// e.g.: credID=cattle-global-data/my-cred headers=X-Token=token;X-User=user
	// the proxy applies credentials according to this server-defined pattern.
	// +optional
	CredentialInjection *CredentialInjectionSpec `json:"credentialInjection,omitempty"`
}

// CredentialInjectionSpec defines how a credential secret's values are injected into a proxied request.

// +kubebuilder:validation:XValidation:rule="(self.mode != 'headerinject' && self.mode != 'bodyinject') || (has(self.fields) && size(self.fields) > 0)",message="fields is required when mode is headerinject or bodyinject"
// +kubebuilder:validation:XValidation:rule="self.mode != 'bearer' || has(self.tokenField) && size(self.tokenField) > 0",message="tokenField is required when mode is bearer"
// +kubebuilder:validation:XValidation:rule="self.mode != 'basic' || (has(self.usernameField) && size(self.usernameField) > 0 && has(self.passwordField) && size(self.passwordField) > 0)",message="usernameField and passwordField are required when mode is basic"
type CredentialInjectionSpec struct {
	// Mode controls how the credential is applied to the request.
	// "bearer"      – sets Authorization: Bearer <token>
	// "basic"       – sets Authorization: Basic base64(username:password)
	// "headerinject" – sets one or more arbitrary request headers
	// "bodyinject"  – merges fields into the top-level JSON request body
	// +required
	// +kubebuilder:validation:Enum=bearer;basic;headerinject;bodyinject
	Mode string `json:"mode"`

	// TokenField is the key within the credential secret whose value is used as the Bearer token.
	// Required when Mode is "bearer".
	// +optional
	TokenField string `json:"tokenField,omitempty"`

	// UsernameField is the key within the credential secret whose value is used as the Basic-auth username.
	// Required when Mode is "basic".
	// +optional
	UsernameField string `json:"usernameField,omitempty"`

	// PasswordField is the key within the credential secret whose value is used as the Basic-auth password.
	// Required when Mode is "basic".
	// +optional
	PasswordField string `json:"passwordField,omitempty"`

	// Fields maps credential secret keys to header names (headerinject) or JSON body keys (bodyinject).
	// Required when Mode is "headerinject" or "bodyinject".
	// +optional
	Fields []InjectionFieldMapping `json:"fields,omitempty"`
}

// InjectionFieldMapping pairs a destination key (header name or JSON body key) with the name
// of the field to read from the credential secret.
type InjectionFieldMapping struct {
	// Key is the header name (for headerinject) or the top-level JSON body key (for bodyinject).
	// +required
	Key string `json:"key"`

	// SecretField is the name of the field within the credential secret to read the value from.
	// For secrets created via the cloudCredential API the field name is the portion after the
	// config-type prefix (e.g. for "genericConfig-apiKey" the SecretField is "apiKey").
	// +required
	SecretField string `json:"secretField"`
}

// SecretReference refers to a Kubernetes secret in the same namespace.
type SecretReference struct {
	// Name is the name of the secret object.
	// +required
	Name string `json:"name"`
}

// TLSVerificationSpec controls certificate verification behavior for TLS connections.
type TLSVerificationSpec struct {
	// VerifyHostname controls whether the certificate's hostname matches the connection hostname.
	// Defaults to true. Only has effect when InsecureSkipTLSVerify is false.
	// +optional
	// +kubebuilder:validation:default=true
	VerifyHostname *bool `json:"verifyHostname,omitempty"`

	// VerifyExpiration controls whether the certificate's validity period is checked.
	// Defaults to true. Only has effect when InsecureSkipTLSVerify is false.
	// +optional
	// +kubebuilder:validation:default=true
	VerifyExpiration *bool `json:"verifyExpiration,omitempty"`
}

type ProxyEndpointStatus struct{}
