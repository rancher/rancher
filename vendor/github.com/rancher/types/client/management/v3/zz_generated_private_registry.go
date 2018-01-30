package client

const (
	PrivateRegistryType          = "privateRegistry"
	PrivateRegistryFieldPassword = "password"
	PrivateRegistryFieldURL      = "url"
	PrivateRegistryFieldUser     = "user"
)

type PrivateRegistry struct {
	Password string `json:"password,omitempty"`
	URL      string `json:"url,omitempty"`
	User     string `json:"user,omitempty"`
}
