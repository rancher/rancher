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

type ProxyEndpointRoute struct {
	// Domain is the domain to be added to the proxy allowlist.
	// Absolute domain names (e.g., example.com) and wildcard patterns are supported.
	// There are two types of supported wildcard patterns:
	//
	// 1) Prefix wildcard (*): Matches any subdomain or prefix. Can only appear as the
	//    leftmost character of the domain. For example:
	//    - *.example.com matches foo.example.com and bar.example.com
	//    - *test.com matches footest.com and bartest.com
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

	// CredentialInjection defines how credentials are applied to proxied requests for this domain.
	// When set, clients only need to supply a credential ID via X-API-CattleAuth-Header;
	// the proxy applies credentials according to this server-defined pattern.
	// +optional
	CredentialInjection *CredentialInjectionSpec `json:"credentialInjection,omitempty"`
}

// CredentialInjectionSpec defines how a credential secret's values are injected into a proxied request.
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

type ProxyEndpointStatus struct{}
