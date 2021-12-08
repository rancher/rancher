package client

const (
	AuditLogConfigType           = "auditLogConfig"
	AuditLogConfigFieldFormat    = "format"
	AuditLogConfigFieldMaxAge    = "maxAge"
	AuditLogConfigFieldMaxBackup = "maxBackup"
	AuditLogConfigFieldMaxSize   = "maxSize"
	AuditLogConfigFieldPath      = "path"
	AuditLogConfigFieldPolicy    = "policy"
)

type AuditLogConfig struct {
	Format    string                 `json:"format,omitempty" yaml:"format,omitempty"`
	MaxAge    int64                  `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	MaxBackup int64                  `json:"maxBackup,omitempty" yaml:"maxBackup,omitempty"`
	MaxSize   int64                  `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	Path      string                 `json:"path,omitempty" yaml:"path,omitempty"`
	Policy    map[string]interface{} `json:"policy,omitempty" yaml:"policy,omitempty"`
}
