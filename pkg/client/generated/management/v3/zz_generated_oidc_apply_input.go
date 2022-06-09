package client

const (
	OIDCApplyInputType            = "oidcApplyInput"
	OIDCApplyInputFieldCode       = "code"
	OIDCApplyInputFieldEnabled    = "enabled"
	OIDCApplyInputFieldOIDCConfig = "oidcConfig"
)

type OIDCApplyInput struct {
	Code       string      `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty" yaml:"oidcConfig,omitempty"`
}
