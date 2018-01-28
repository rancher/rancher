package client

const (
	SyslogConfigType          = "syslogConfig"
	SyslogConfigFieldHost     = "host"
	SyslogConfigFieldPort     = "port"
	SyslogConfigFieldProgram  = "program"
	SyslogConfigFieldSeverity = "severity"
)

type SyslogConfig struct {
	Host     string `json:"host,omitempty"`
	Port     *int64 `json:"port,omitempty"`
	Program  string `json:"program,omitempty"`
	Severity string `json:"severity,omitempty"`
}
