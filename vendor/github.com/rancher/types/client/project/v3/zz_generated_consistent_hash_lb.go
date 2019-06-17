package client

const (
	ConsistentHashLBType                 = "consistentHashLB"
	ConsistentHashLBFieldHTTPCookie      = "httpCookie"
	ConsistentHashLBFieldHTTPHeaderName  = "httpHeaderName"
	ConsistentHashLBFieldMinimumRingSize = "minimumRingSize"
	ConsistentHashLBFieldUseSourceIP     = "useSourceIp"
)

type ConsistentHashLB struct {
	HTTPCookie      *HTTPCookie `json:"httpCookie,omitempty" yaml:"httpCookie,omitempty"`
	HTTPHeaderName  string      `json:"httpHeaderName,omitempty" yaml:"httpHeaderName,omitempty"`
	MinimumRingSize int64       `json:"minimumRingSize,omitempty" yaml:"minimumRingSize,omitempty"`
	UseSourceIP     bool        `json:"useSourceIp,omitempty" yaml:"useSourceIp,omitempty"`
}
