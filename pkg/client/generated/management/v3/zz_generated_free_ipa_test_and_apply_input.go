package client

const (
	FreeIpaTestAndApplyInputType            = "freeIpaTestAndApplyInput"
	FreeIpaTestAndApplyInputFieldConfigName = "configName"
	FreeIpaTestAndApplyInputFieldLdapConfig = "ldapConfig"
	FreeIpaTestAndApplyInputFieldPassword   = "password"
	FreeIpaTestAndApplyInputFieldUsername   = "username"
)

type FreeIpaTestAndApplyInput struct {
	ConfigName string      `json:"configName,omitempty" yaml:"configName,omitempty"`
	LdapConfig *LdapConfig `json:"ldapConfig,omitempty" yaml:"ldapConfig,omitempty"`
	Password   string      `json:"password,omitempty" yaml:"password,omitempty"`
	Username   string      `json:"username,omitempty" yaml:"username,omitempty"`
}
