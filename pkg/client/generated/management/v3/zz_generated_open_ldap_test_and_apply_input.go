package client

const (
	OpenLdapTestAndApplyInputType            = "openLdapTestAndApplyInput"
	OpenLdapTestAndApplyInputFieldConfigName = "configName"
	OpenLdapTestAndApplyInputFieldLdapConfig = "ldapConfig"
	OpenLdapTestAndApplyInputFieldPassword   = "password"
	OpenLdapTestAndApplyInputFieldUsername   = "username"
)

type OpenLdapTestAndApplyInput struct {
	ConfigName string      `json:"configName,omitempty" yaml:"configName,omitempty"`
	LdapConfig *LdapConfig `json:"ldapConfig,omitempty" yaml:"ldapConfig,omitempty"`
	Password   string      `json:"password,omitempty" yaml:"password,omitempty"`
	Username   string      `json:"username,omitempty" yaml:"username,omitempty"`
}
