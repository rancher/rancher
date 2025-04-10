package client

const (
	ProjectStatusType                               = "projectStatus"
	ProjectStatusFieldBackingNamespace              = "backingNamespace"
	ProjectStatusFieldConditions                    = "conditions"
	ProjectStatusFieldMonitoringStatus              = "monitoringStatus"
	ProjectStatusFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
)

type ProjectStatus struct {
	BackingNamespace              string             `json:"backingNamespace,omitempty" yaml:"backingNamespace,omitempty"`
	Conditions                    []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	MonitoringStatus              *MonitoringStatus  `json:"monitoringStatus,omitempty" yaml:"monitoringStatus,omitempty"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
}
