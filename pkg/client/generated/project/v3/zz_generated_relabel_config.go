package client

const (
	RelabelConfigType              = "relabelConfig"
	RelabelConfigFieldAction       = "action"
	RelabelConfigFieldModulus      = "modulus"
	RelabelConfigFieldRegex        = "regex"
	RelabelConfigFieldReplacement  = "replacement"
	RelabelConfigFieldSeparator    = "separator"
	RelabelConfigFieldSourceLabels = "sourceLabels"
	RelabelConfigFieldTargetLabel  = "targetLabel"
)

type RelabelConfig struct {
	Action       string   `json:"action,omitempty" yaml:"action,omitempty"`
	Modulus      int64    `json:"modulus,omitempty" yaml:"modulus,omitempty"`
	Regex        string   `json:"regex,omitempty" yaml:"regex,omitempty"`
	Replacement  string   `json:"replacement,omitempty" yaml:"replacement,omitempty"`
	Separator    string   `json:"separator,omitempty" yaml:"separator,omitempty"`
	SourceLabels []string `json:"sourceLabels,omitempty" yaml:"sourceLabels,omitempty"`
	TargetLabel  string   `json:"targetLabel,omitempty" yaml:"targetLabel,omitempty"`
}
