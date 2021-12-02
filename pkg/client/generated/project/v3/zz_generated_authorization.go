package client

const (
	AuthorizationType                 = "authorization"
	AuthorizationFieldCredentials     = "credentials"
	AuthorizationFieldCredentialsFile = "credentialsFile"
	AuthorizationFieldType            = "type"
)

type Authorization struct {
	Credentials     *SecretKeySelector `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	CredentialsFile string             `json:"credentialsFile,omitempty" yaml:"credentialsFile,omitempty"`
	Type            string             `json:"type,omitempty" yaml:"type,omitempty"`
}
