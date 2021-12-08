package client

const (
	FreeIpaTestAndApplyInputType            = "freeIpaTestAndApplyInput"
	FreeIpaTestAndApplyInputFieldLdapConfig = "ldapConfig"
	FreeIpaTestAndApplyInputFieldPassword   = "password"
	FreeIpaTestAndApplyInputFieldUsername   = "username"
)

type FreeIpaTestAndApplyInput struct {
	LdapConfig *LdapConfig `json:"ldapConfig,omitempty" yaml:"ldapConfig,omitempty"`
	Password   string      `json:"password,omitempty" yaml:"password,omitempty"`
	Username   string      `json:"username,omitempty" yaml:"username,omitempty"`
}
