package client

const (
	PolicyRuleType                 = "policyRule"
	PolicyRuleFieldAPIGroups       = "apiGroups"
	PolicyRuleFieldNonResourceURLs = "nonResourceURLs"
	PolicyRuleFieldResourceNames   = "resourceNames"
	PolicyRuleFieldResources       = "resources"
	PolicyRuleFieldVerbs           = "verbs"
)

type PolicyRule struct {
	APIGroups       []string `json:"apiGroups,omitempty" yaml:"apiGroups,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty" yaml:"nonResourceURLs,omitempty"`
	ResourceNames   []string `json:"resourceNames,omitempty" yaml:"resourceNames,omitempty"`
	Resources       []string `json:"resources,omitempty" yaml:"resources,omitempty"`
	Verbs           []string `json:"verbs,omitempty" yaml:"verbs,omitempty"`
}
