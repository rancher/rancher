package client

const (
	RegistryCredentialType             = "registryCredential"
	RegistryCredentialFieldAuth        = "auth"
	RegistryCredentialFieldDescription = "description"
	RegistryCredentialFieldEmail       = "email"
	RegistryCredentialFieldPassword    = "password"
	RegistryCredentialFieldUsername    = "username"
)

type RegistryCredential struct {
	Auth        string `json:"auth,omitempty" yaml:"auth,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Email       string `json:"email,omitempty" yaml:"email,omitempty"`
	Password    string `json:"password,omitempty" yaml:"password,omitempty"`
	Username    string `json:"username,omitempty" yaml:"username,omitempty"`
}
