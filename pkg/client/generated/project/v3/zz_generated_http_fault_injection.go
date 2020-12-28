package client

const (
	HTTPFaultInjectionType       = "httpFaultInjection"
	HTTPFaultInjectionFieldAbort = "abort"
	HTTPFaultInjectionFieldDelay = "delay"
)

type HTTPFaultInjection struct {
	Abort *InjectAbort `json:"abort,omitempty" yaml:"abort,omitempty"`
	Delay *InjectDelay `json:"delay,omitempty" yaml:"delay,omitempty"`
}
