package client

const (
	OAuth2Type                = "oAuth2"
	OAuth2FieldClientID       = "clientId"
	OAuth2FieldClientSecret   = "clientSecret"
	OAuth2FieldEndpointParams = "endpointParams"
	OAuth2FieldScopes         = "scopes"
	OAuth2FieldTokenURL       = "tokenUrl"
)

type OAuth2 struct {
	ClientID       *SecretOrConfigMap `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	ClientSecret   *SecretKeySelector `json:"clientSecret,omitempty" yaml:"clientSecret,omitempty"`
	EndpointParams map[string]string  `json:"endpointParams,omitempty" yaml:"endpointParams,omitempty"`
	Scopes         []string           `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	TokenURL       string             `json:"tokenUrl,omitempty" yaml:"tokenUrl,omitempty"`
}
