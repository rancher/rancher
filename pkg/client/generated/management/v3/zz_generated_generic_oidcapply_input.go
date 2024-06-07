package client

const (
	GenericOIDCApplyInputType            = "genericOIDCApplyInput"
	GenericOIDCApplyInputFieldCode       = "code"
	GenericOIDCApplyInputFieldEnabled    = "enabled"
	GenericOIDCApplyInputFieldOIDCConfig = "oidcConfig"
)

type GenericOIDCApplyInput struct {
	Code       string      `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty" yaml:"oidcConfig,omitempty"`
}
