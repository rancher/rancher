package client

const (
	OIDCApplyInputType            = "oidcApplyInput"
	OIDCApplyInputFieldCode       = "code"
	OIDCApplyInputFieldConfigName = "configName"
	OIDCApplyInputFieldEnabled    = "enabled"
	OIDCApplyInputFieldOIDCConfig = "oidcConfig"
)

type OIDCApplyInput struct {
	Code       string      `json:"code,omitempty" yaml:"code,omitempty"`
	ConfigName string      `json:"configName,omitempty" yaml:"configName,omitempty"`
	Enabled    bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OIDCConfig *OIDCConfig `json:"oidcConfig,omitempty" yaml:"oidcConfig,omitempty"`
}
