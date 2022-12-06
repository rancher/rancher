package client

const (
	PodSecurityAdmissionConfigurationTemplateExemptionsType                = "podSecurityAdmissionConfigurationTemplateExemptions"
	PodSecurityAdmissionConfigurationTemplateExemptionsFieldNamespaces     = "namespaces"
	PodSecurityAdmissionConfigurationTemplateExemptionsFieldRuntimeClasses = "runtimeClasses"
	PodSecurityAdmissionConfigurationTemplateExemptionsFieldUsernames      = "usernames"
)

type PodSecurityAdmissionConfigurationTemplateExemptions struct {
	Namespaces     []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	RuntimeClasses []string `json:"runtimeClasses,omitempty" yaml:"runtimeClasses,omitempty"`
	Usernames      []string `json:"usernames,omitempty" yaml:"usernames,omitempty"`
}
