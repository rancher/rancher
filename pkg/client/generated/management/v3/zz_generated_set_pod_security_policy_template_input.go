package client

const (
	SetPodSecurityPolicyTemplateInputType                               = "setPodSecurityPolicyTemplateInput"
	SetPodSecurityPolicyTemplateInputFieldPodSecurityPolicyTemplateName = "podSecurityPolicyTemplateId"
)

type SetPodSecurityPolicyTemplateInput struct {
	PodSecurityPolicyTemplateName string `json:"podSecurityPolicyTemplateId,omitempty" yaml:"podSecurityPolicyTemplateId,omitempty"`
}
