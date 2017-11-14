package client

const (
	ProbeType                     = "probe"
	ProbeFieldCommand             = "command"
	ProbeFieldFailureThreshold    = "failureThreshold"
	ProbeFieldHTTPHeaders         = "httpHeaders"
	ProbeFieldHost                = "host"
	ProbeFieldInitialDelaySeconds = "initialDelaySeconds"
	ProbeFieldPath                = "path"
	ProbeFieldPeriodSeconds       = "periodSeconds"
	ProbeFieldPort                = "port"
	ProbeFieldScheme              = "scheme"
	ProbeFieldSuccessThreshold    = "successThreshold"
	ProbeFieldTCP                 = "tcp"
	ProbeFieldTimeoutSeconds      = "timeoutSeconds"
)

type Probe struct {
	Command             []string     `json:"command,omitempty"`
	FailureThreshold    *int64       `json:"failureThreshold,omitempty"`
	HTTPHeaders         []HTTPHeader `json:"httpHeaders,omitempty"`
	Host                string       `json:"host,omitempty"`
	InitialDelaySeconds *int64       `json:"initialDelaySeconds,omitempty"`
	Path                string       `json:"path,omitempty"`
	PeriodSeconds       *int64       `json:"periodSeconds,omitempty"`
	Port                string       `json:"port,omitempty"`
	Scheme              string       `json:"scheme,omitempty"`
	SuccessThreshold    *int64       `json:"successThreshold,omitempty"`
	TCP                 *bool        `json:"tcp,omitempty"`
	TimeoutSeconds      *int64       `json:"timeoutSeconds,omitempty"`
}
