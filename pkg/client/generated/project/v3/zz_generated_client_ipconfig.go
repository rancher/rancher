package client

const (
	ClientIPConfigType                = "clientIPConfig"
	ClientIPConfigFieldTimeoutSeconds = "timeoutSeconds"
)

type ClientIPConfig struct {
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}
