package client

const (
	SafeAuthorizationType             = "safeAuthorization"
	SafeAuthorizationFieldCredentials = "credentials"
	SafeAuthorizationFieldType        = "type"
)

type SafeAuthorization struct {
	Credentials *SecretKeySelector `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	Type        string             `json:"type,omitempty" yaml:"type,omitempty"`
}
