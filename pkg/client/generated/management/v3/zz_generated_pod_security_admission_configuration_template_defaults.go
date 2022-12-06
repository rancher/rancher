package client

const (
	PodSecurityAdmissionConfigurationTemplateDefaultsType                = "podSecurityAdmissionConfigurationTemplateDefaults"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldAudit          = "audit"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldAuditVersion   = "audit-version"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldEnforce        = "enforce"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldEnforceVersion = "enforce-version"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldWarn           = "warn"
	PodSecurityAdmissionConfigurationTemplateDefaultsFieldWarnVersion    = "warn-version"
)

type PodSecurityAdmissionConfigurationTemplateDefaults struct {
	Audit          string `json:"audit,omitempty" yaml:"audit,omitempty"`
	AuditVersion   string `json:"audit-version,omitempty" yaml:"audit-version,omitempty"`
	Enforce        string `json:"enforce,omitempty" yaml:"enforce,omitempty"`
	EnforceVersion string `json:"enforce-version,omitempty" yaml:"enforce-version,omitempty"`
	Warn           string `json:"warn,omitempty" yaml:"warn,omitempty"`
	WarnVersion    string `json:"warn-version,omitempty" yaml:"warn-version,omitempty"`
}
