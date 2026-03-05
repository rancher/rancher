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
}

type ProxyEndpointStatus struct{}
