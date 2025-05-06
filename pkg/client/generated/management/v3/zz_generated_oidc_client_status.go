package client

const (
	OIDCClientStatusType               = "oidcClientStatus"
	OIDCClientStatusFieldClientID      = "clientID"
	OIDCClientStatusFieldClientSecrets = "clientSecrets"
)

type OIDCClientStatus struct {
	ClientID      string                            `json:"clientID,omitempty" yaml:"clientID,omitempty"`
	ClientSecrets map[string]OIDCClientSecretStatus `json:"clientSecrets,omitempty" yaml:"clientSecrets,omitempty"`
}
