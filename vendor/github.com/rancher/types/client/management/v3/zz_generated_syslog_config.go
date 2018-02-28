package client

const (
	SyslogConfigType          = "syslogConfig"
	SyslogConfigFieldEndpoint = "endpoint"
	SyslogConfigFieldProgram  = "program"
	SyslogConfigFieldProtocol = "protocol"
	SyslogConfigFieldSeverity = "severity"
)

type SyslogConfig struct {
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Program  string `json:"program,omitempty" yaml:"program,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Severity string `json:"severity,omitempty" yaml:"severity,omitempty"`
}
