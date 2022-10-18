package client

const (
	AuditLogType               = "auditLog"
	AuditLogFieldConfiguration = "configuration"
	AuditLogFieldEnabled       = "enabled"
)

type AuditLog struct {
	Configuration *AuditLogConfig `json:"configuration,omitempty" yaml:"configuration,omitempty"`
	Enabled       bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}
