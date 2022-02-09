package client

const (
	MetadataConfigType              = "metadataConfig"
	MetadataConfigFieldSend         = "send"
	MetadataConfigFieldSendInterval = "sendInterval"
)

type MetadataConfig struct {
	Send         bool   `json:"send,omitempty" yaml:"send,omitempty"`
	SendInterval string `json:"sendInterval,omitempty" yaml:"sendInterval,omitempty"`
}
