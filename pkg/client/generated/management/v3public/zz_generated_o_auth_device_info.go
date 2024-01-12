package client

const (
	OAuthDeviceInfoType              = "oAuthDeviceInfo"
	OAuthDeviceInfoFieldClientID     = "clientId"
	OAuthDeviceInfoFieldClientSecret = "clientSecret"
)

type OAuthDeviceInfo struct {
	ClientID     string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
}
