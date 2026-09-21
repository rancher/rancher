package client

const (
	KeyCloakOIDCApplyInputType            = "keyCloakOIDCApplyInput"
	KeyCloakOIDCApplyInputFieldCode       = "code"
	KeyCloakOIDCApplyInputFieldEnabled    = "enabled"
	KeyCloakOIDCApplyInputFieldOIDCConfig = "oidcConfig"
)

type KeyCloakOIDCApplyInput struct {
	Code       string              `json:"code,omitempty" yaml:"code,omitempty"`
	Enabled    bool                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OIDCConfig *KeyCloakOIDCConfig `json:"oidcConfig,omitempty" yaml:"oidcConfig,omitempty"`
}
