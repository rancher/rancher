package client

const (
	OIDCClientStatusType          = "oidcClientStatus"
	OIDCClientStatusFieldClientID = "clientID"
)

type OIDCClientStatus struct {
	ClientID string `json:"clientID,omitempty" yaml:"clientID,omitempty"`
}
