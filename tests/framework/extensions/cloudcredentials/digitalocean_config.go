package cloudcredentials

const DigitalOceanCredentialConfigurationFileKey = "digitalOceanCredentials"

type DigitalOceanCredentialConfig struct {
	AccessToken string `json:"accessToken" yaml:"accessToken"`
}
