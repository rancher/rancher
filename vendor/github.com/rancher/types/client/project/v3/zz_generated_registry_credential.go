package client

const (
	RegistryCredentialType             = "registryCredential"
	RegistryCredentialFieldAuth        = "auth"
	RegistryCredentialFieldDescription = "description"
	RegistryCredentialFieldPassword    = "password"
	RegistryCredentialFieldUsername    = "username"
)

type RegistryCredential struct {
	Auth        string `json:"auth,omitempty"`
	Description string `json:"description,omitempty"`
	Password    string `json:"password,omitempty"`
	Username    string `json:"username,omitempty"`
}
