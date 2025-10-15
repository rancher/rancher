package client

const (
	AzureADLoginType              = "azureADLogin"
	AzureADLoginFieldCode         = "code"
	AzureADLoginFieldDescription  = "description"
	AzureADLoginFieldIDToken      = "id_token"
	AzureADLoginFieldResponseType = "responseType"
	AzureADLoginFieldTTLMillis    = "ttl"
	AzureADLoginFieldType         = "type"
)

type AzureADLogin struct {
	Code         string `json:"code,omitempty" yaml:"code,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	IDToken      string `json:"id_token,omitempty" yaml:"id_token,omitempty"`
	ResponseType string `json:"responseType,omitempty" yaml:"responseType,omitempty"`
	TTLMillis    int64  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Type         string `json:"type,omitempty" yaml:"type,omitempty"`
}
