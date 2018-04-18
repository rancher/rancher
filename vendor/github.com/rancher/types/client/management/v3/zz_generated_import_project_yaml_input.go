package client

const (
	ImportProjectYamlInputType           = "importProjectYamlInput"
	ImportProjectYamlInputFieldNamespace = "namespace"
	ImportProjectYamlInputFieldYaml      = "yaml"
)

type ImportProjectYamlInput struct {
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Yaml      string `json:"yaml,omitempty" yaml:"yaml,omitempty"`
}
