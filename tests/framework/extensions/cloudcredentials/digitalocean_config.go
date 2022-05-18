package cloudcredentials

// The json/yaml config key for the digital ocean cloud credential config
const DigitalOceanCredentialConfigurationFileKey = "digitalOceanCredentials"

// DigitalOceanCredentialConfig is configuration need to create a digital ocean cloud credential
type DigitalOceanCredentialConfig struct {
	AccessToken string `json:"accessToken" yaml:"accessToken"`
}
