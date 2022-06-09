package client

const (
	PrivateRegistryType                     = "privateRegistry"
	PrivateRegistryFieldECRCredentialPlugin = "ecrCredentialPlugin"
	PrivateRegistryFieldIsDefault           = "isDefault"
	PrivateRegistryFieldPassword            = "password"
	PrivateRegistryFieldURL                 = "url"
	PrivateRegistryFieldUser                = "user"
)

type PrivateRegistry struct {
	ECRCredentialPlugin *ECRCredentialPlugin `json:"ecrCredentialPlugin,omitempty" yaml:"ecrCredentialPlugin,omitempty"`
	IsDefault           bool                 `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`
	Password            string               `json:"password,omitempty" yaml:"password,omitempty"`
	URL                 string               `json:"url,omitempty" yaml:"url,omitempty"`
	User                string               `json:"user,omitempty" yaml:"user,omitempty"`
}
