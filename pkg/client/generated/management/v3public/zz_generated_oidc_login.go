package client

const (
	OIDCLoginType              = "oidcLogin"
	OIDCLoginFieldCode         = "code"
	OIDCLoginFieldConfigName   = "configName"
	OIDCLoginFieldDescription  = "description"
	OIDCLoginFieldResponseType = "responseType"
	OIDCLoginFieldTTLMillis    = "ttl"
	OIDCLoginFieldType         = "type"
)

type OIDCLogin struct {
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	ConfigName   string `json:"configName,omitempty" yaml:"configName,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Type         string `json:"type,omitempty" yaml:"type,omitempty"`
}
