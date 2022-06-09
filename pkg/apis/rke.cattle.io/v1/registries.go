package v1

// Mirror contains the config related to the registry mirror
type Mirror struct {
	// Endpoints are endpoints for a namespace. CRI plugin will try the endpoints
	// one by one until a working one is found. The endpoint must be a valid url
	// with host specified.
	// The scheme, host and path from the endpoint URL will be used.
	Endpoints []string `json:"endpoint,omitempty"`

	// Rewrites are repository rewrite rules for a namespace. When fetching image resources
	// from an endpoint and a key matches the repository via regular expression matching
	// it will be replaced with the corresponding value from the map in the resource request.
	Rewrites map[string]string `json:"rewrite,omitempty"`
}

const (
	AuthConfigSecretType = "rke.cattle.io/auth-config"

	UsernameAuthConfigSecretKey      = "username"
	PasswordAuthConfigSecretKey      = "password"
	AuthAuthConfigSecretKey          = "auth"
	IdentityTokenAuthConfigSecretKey = "identityToken"
)

// AuthConfig contains the config related to authentication to a specific registry
type AuthConfig struct {
	// Username is the username to login the registry.
	Username string `json:"username,omitempty"`
	// Password is the password to login the registry.
	Password string `json:"password,omitempty"`
	// Auth is a base64 encoded string from the concatenation of the username,
	// a colon, and the password.
	Auth string `json:"auth,omitempty"`
	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identityToken,omitempty"`
}

// Registry is registry settings configured
type Registry struct {
	// Mirrors are namespace to mirror mapping for all namespaces.
	Mirrors map[string]Mirror `json:"mirrors,omitempty"`
	// Configs are configs for each registry.
	// The key is the FDQN or IP of the registry.
	Configs map[string]RegistryConfig `json:"configs,omitempty"`
}

// RegistryConfig contains configuration used to communicate with the registry.
type RegistryConfig struct {
	// Auth contains information to authenticate to the registry.
	AuthConfigSecretName string `json:"authConfigSecretName,omitempty"`
	// TLS is a pair of Cert/Key which then are used when creating the transport
	// that communicates with the registry.
	TLSSecretName string `json:"tlsSecretName,omitempty"`
	CABundle      []byte `json:"caBundle,omitempty"`

	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
