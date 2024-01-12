package client

const (
	OAuthEndpointType               = "oAuthEndpoint"
	OAuthEndpointFieldAuthURL       = "authUrl"
	OAuthEndpointFieldDeviceAuthURL = "deviceAuthUrl"
	OAuthEndpointFieldTokenURL      = "tokenUrl"
)

type OAuthEndpoint struct {
	AuthURL       string `json:"authUrl,omitempty" yaml:"authUrl,omitempty"`
	DeviceAuthURL string `json:"deviceAuthUrl,omitempty" yaml:"deviceAuthUrl,omitempty"`
	TokenURL      string `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
}
