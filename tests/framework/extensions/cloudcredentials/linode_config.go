package cloudcredentials

// The json/yaml config key for the linode cloud credential config
const LinodeCredentialConfigurationFileKey = "linodeCredentials"

// LinodeCredentialConfig is configuration need to create a linode cloud credential
type LinodeCredentialConfig struct {
	Token string `json:"token" yaml:"token"`
}
