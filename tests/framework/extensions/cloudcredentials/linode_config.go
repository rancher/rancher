package cloudcredentials

const LinodeCredentialConfigurationFileKey = "linodeCredentials"

type LinodeCredentialConfig struct {
	Token string `json:"token" yaml:"token"`
}
