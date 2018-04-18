package client

const (
	ImportClusterYamlInputType      = "importClusterYamlInput"
	ImportClusterYamlInputFieldYaml = "yaml"
)

type ImportClusterYamlInput struct {
	Yaml string `json:"yaml,omitempty" yaml:"yaml,omitempty"`
}
