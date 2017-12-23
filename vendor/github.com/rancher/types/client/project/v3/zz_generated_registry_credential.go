package client

const (
	RegistryCredentialType          = "registryCredential"
	RegistryCredentialFieldAuth     = "auth"
	RegistryCredentialFieldPassword = "password"
	RegistryCredentialFieldUsername = "username"
)

type RegistryCredential struct {
	Auth     string `json:"auth,omitempty"`
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
}
