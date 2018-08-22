package client

const (
	LdapLoginType              = "ldapLogin"
	LdapLoginFieldDescription  = "description"
	LdapLoginFieldPassword     = "password"
	LdapLoginFieldResponseType = "responseType"
	LdapLoginFieldTTLMillis    = "ttl"
	LdapLoginFieldUserUniqueID = "userUniqueId"
	LdapLoginFieldUsername     = "username"
)

type LdapLogin struct {
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	UserUniqueID string `json:"userUniqueId,omitempty" yaml:"userUniqueId,omitempty"`
	Username     string `json:"username,omitempty" yaml:"username,omitempty"`
}
