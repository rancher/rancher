package client

const (
	OIDCClientSpecType                               = "oidcClientSpec"
	OIDCClientSpecFieldDescription                   = "description"
	OIDCClientSpecFieldRedirectURIs                  = "redirectURIs"
	OIDCClientSpecFieldRefreshTokenExpirationSeconds = "refreshTokenExpirationSeconds"
	OIDCClientSpecFieldScopes                        = "scopes"
	OIDCClientSpecFieldTokenExpirationSeconds        = "tokenExpirationSeconds"
)

type OIDCClientSpec struct {
	Description                   string   `json:"description,omitempty" yaml:"description,omitempty"`
	RedirectURIs                  []string `json:"redirectURIs,omitempty" yaml:"redirectURIs,omitempty"`
	RefreshTokenExpirationSeconds int64    `json:"refreshTokenExpirationSeconds,omitempty" yaml:"refreshTokenExpirationSeconds,omitempty"`
	Scopes                        []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	TokenExpirationSeconds        int64    `json:"tokenExpirationSeconds,omitempty" yaml:"tokenExpirationSeconds,omitempty"`
}
