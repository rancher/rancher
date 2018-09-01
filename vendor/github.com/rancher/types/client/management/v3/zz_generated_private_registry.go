package client

const (
	PrivateRegistryType           = "privateRegistry"
	PrivateRegistryFieldIsDefault = "isDefault"
	PrivateRegistryFieldPassword  = "password"
	PrivateRegistryFieldURL       = "url"
	PrivateRegistryFieldUser      = "user"
)

type PrivateRegistry struct {
	IsDefault bool   `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty"`
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	User      string `json:"user,omitempty" yaml:"user,omitempty"`
}
