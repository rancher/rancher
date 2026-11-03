package client

const (
	GenericOIDCApplyInputType            = "genericOIDCApplyInput"
	GenericOIDCApplyInputFieldCode       = "code"
	GenericOIDCApplyInputFieldConfigName = "configName"
	GenericOIDCApplyInputFieldEnabled    = "enabled"
	GenericOIDCApplyInputFieldOIDCConfig = "oidcConfig"
)

type GenericOIDCApplyInput struct {
	Code       string      `json:"code,omitempty" yaml:"code,omitempty"`
	ConfigName string      `json:"configName,omitempty" yaml:"configName,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty" yaml:"oidcConfig,omitempty"`
}
