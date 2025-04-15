package client

const (
	ProjectStatusType                               = "projectStatus"
	ProjectStatusFieldConditions                    = "conditions"
	ProjectStatusFieldMonitoringStatus              = "monitoringStatus"
	ProjectStatusFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
)

type ProjectStatus struct {
	Conditions                    []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	MonitoringStatus              *MonitoringStatus  `json:"monitoringStatus,omitempty" yaml:"monitoringStatus,omitempty"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
}
