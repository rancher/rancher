package client

const (
	EmbeddedConfigType             = "embeddedConfig"
	EmbeddedConfigFieldDateFormat  = "dateFormat"
	EmbeddedConfigFieldIndexPrefix = "indexPrefix"
)

type EmbeddedConfig struct {
	DateFormat  string `json:"dateFormat,omitempty"`
	IndexPrefix string `json:"indexPrefix,omitempty"`
}
