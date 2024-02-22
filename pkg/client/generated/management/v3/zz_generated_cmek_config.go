package client

const (
	CMEKConfigType          = "cmekConfig"
	CMEKConfigFieldKeyName  = "keyName"
	CMEKConfigFieldRingName = "ringName"
)

type CMEKConfig struct {
	KeyName  string `json:"keyName,omitempty" yaml:"keyName,omitempty"`
	RingName string `json:"ringName,omitempty" yaml:"ringName,omitempty"`
}
