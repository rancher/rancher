package client

const (
	PodSecurityAdmissionConfigurationTemplateSpecType            = "podSecurityAdmissionConfigurationTemplateSpec"
	PodSecurityAdmissionConfigurationTemplateSpecFieldDefaults   = "defaults"
	PodSecurityAdmissionConfigurationTemplateSpecFieldExemptions = "exemptions"
)

type PodSecurityAdmissionConfigurationTemplateSpec struct {
	Defaults   *PodSecurityAdmissionConfigurationTemplateDefaults   `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Exemptions *PodSecurityAdmissionConfigurationTemplateExemptions `json:"exemptions,omitempty" yaml:"exemptions,omitempty"`
}
