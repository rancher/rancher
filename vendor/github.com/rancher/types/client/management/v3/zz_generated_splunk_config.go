package client

const (
	SplunkConfigType          = "splunkConfig"
	SplunkConfigFieldEndpoint = "endpoint"
	SplunkConfigFieldSource   = "source"
	SplunkConfigFieldToken    = "token"
)

type SplunkConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
	Source   string `json:"source,omitempty"`
	Token    string `json:"token,omitempty"`
}
