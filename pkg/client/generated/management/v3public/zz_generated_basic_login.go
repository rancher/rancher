package client

const (
	BasicLoginType              = "basicLogin"
	BasicLoginFieldConfigName   = "configName"
	BasicLoginFieldDescription  = "description"
	BasicLoginFieldPassword     = "password"
	BasicLoginFieldResponseType = "responseType"
	BasicLoginFieldTTLMillis    = "ttl"
	BasicLoginFieldType         = "type"
	BasicLoginFieldUsername     = "username"
)

type BasicLogin struct {
	ConfigName   string `json:"configName,omitempty" yaml:"configName,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Type         string `json:"type,omitempty" yaml:"type,omitempty"`
	Username     string `json:"username,omitempty" yaml:"username,omitempty"`
}
