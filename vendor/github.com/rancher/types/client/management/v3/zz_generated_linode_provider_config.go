package client

const (
	LinodeProviderConfigType          = "linodeProviderConfig"
	LinodeProviderConfigFieldAPIToken = "apiToken"
)

type LinodeProviderConfig struct {
	APIToken string `json:"apiToken,omitempty" yaml:"apiToken,omitempty"`
}
