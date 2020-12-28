package client

const (
	ImportYamlOutputType         = "importYamlOutput"
	ImportYamlOutputFieldMessage = "message"
)

type ImportYamlOutput struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}
