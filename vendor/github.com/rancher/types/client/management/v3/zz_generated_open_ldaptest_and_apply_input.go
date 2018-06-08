package client

const (
	OpenLDAPTestAndApplyInputType                = "openLDAPTestAndApplyInput"
	OpenLDAPTestAndApplyInputFieldEnabled        = "enabled"
	OpenLDAPTestAndApplyInputFieldOpenLDAPConfig = "openLDAPConfig"
	OpenLDAPTestAndApplyInputFieldPassword       = "password"
	OpenLDAPTestAndApplyInputFieldUsername       = "username"
)

type OpenLDAPTestAndApplyInput struct {
	Enabled        bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	OpenLDAPConfig *OpenLDAPConfig `json:"openLDAPConfig,omitempty" yaml:"openLDAPConfig,omitempty"`
	Password       string          `json:"password,omitempty" yaml:"password,omitempty"`
	Username       string          `json:"username,omitempty" yaml:"username,omitempty"`
}
