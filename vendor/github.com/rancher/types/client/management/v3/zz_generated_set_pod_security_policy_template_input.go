package client

const (
	SetPodSecurityPolicyTemplateInputType                           = "setPodSecurityPolicyTemplateInput"
	SetPodSecurityPolicyTemplateInputFieldPodSecurityPolicyTemplate = "podSecurityPolicyTemplate"
)

type SetPodSecurityPolicyTemplateInput struct {
	PodSecurityPolicyTemplate string `json:"podSecurityPolicyTemplate,omitempty" yaml:"podSecurityPolicyTemplate,omitempty"`
}
