package client

const (
	SplunkConfigType          = "splunkConfig"
	SplunkConfigFieldHost     = "host"
	SplunkConfigFieldPort     = "port"
	SplunkConfigFieldProtocol = "protocol"
	SplunkConfigFieldSource   = "source"
	SplunkConfigFieldToken    = "token"
)

type SplunkConfig struct {
	Host     string `json:"host,omitempty"`
	Port     *int64 `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Source   string `json:"source,omitempty"`
	Token    string `json:"token,omitempty"`
}
