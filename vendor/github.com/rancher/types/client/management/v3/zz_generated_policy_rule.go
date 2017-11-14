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
	APIGroups       []string `json:"apiGroups,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty"`
	ResourceNames   []string `json:"resourceNames,omitempty"`
	Resources       []string `json:"resources,omitempty"`
	Verbs           []string `json:"verbs,omitempty"`
}
