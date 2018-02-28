package client

const (
	PrivateRegistryType          = "privateRegistry"
	PrivateRegistryFieldPassword = "password"
	PrivateRegistryFieldURL      = "url"
	PrivateRegistryFieldUser     = "user"
)

type PrivateRegistry struct {
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	URL      string `json:"url,omitempty" yaml:"url,omitempty"`
	User     string `json:"user,omitempty" yaml:"user,omitempty"`
}
