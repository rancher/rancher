package client

const (
	ApplyYamlConfigType           = "applyYamlConfig"
	ApplyYamlConfigFieldContent   = "content"
	ApplyYamlConfigFieldNamespace = "namespace"
	ApplyYamlConfigFieldPath      = "path"
)

type ApplyYamlConfig struct {
	Content   string `json:"content,omitempty" yaml:"content,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Path      string `json:"path,omitempty" yaml:"path,omitempty"`
}
