package client

const (
	ProjectStatusType                               = "projectStatus"
	ProjectStatusFieldConditions                    = "conditions"
	ProjectStatusFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
)

type ProjectStatus struct {
	Conditions                    []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	PodSecurityPolicyTemplateName string             `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
}
