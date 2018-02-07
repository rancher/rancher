package client

const (
	SyslogConfigType          = "syslogConfig"
	SyslogConfigFieldEndpoint = "endpoint"
	SyslogConfigFieldProgram  = "program"
	SyslogConfigFieldProtocol = "protocol"
	SyslogConfigFieldSeverity = "severity"
)

type SyslogConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
	Program  string `json:"program,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Severity string `json:"severity,omitempty"`
}
