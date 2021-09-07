package client

const (
	DigitalOceanCredentialConfigType             = "digitalOceanCredentialConfig"
	DigitalOceanCredentialConfigFieldAccessToken = "accessToken"
)

type DigitalOceanCredentialConfig struct {
	AccessToken string `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
}
