package client

const (
	OIDCClientSecretStatusType                    = "oidcClientSecretStatus"
	OIDCClientSecretStatusFieldCreatedAt          = "createdAt"
	OIDCClientSecretStatusFieldLastFiveCharacters = "lastFiveCharacters"
	OIDCClientSecretStatusFieldLastUsedAt         = "lastUsedAt"
)

type OIDCClientSecretStatus struct {
	CreatedAt          string `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	LastFiveCharacters string `json:"lastFiveCharacters,omitempty" yaml:"lastFiveCharacters,omitempty"`
	LastUsedAt         string `json:"lastUsedAt,omitempty" yaml:"lastUsedAt,omitempty"`
}
