package client

const (
	ImportYamlOutputType               = "importYamlOutput"
	ImportYamlOutputFieldOutputMessage = "outputMessage"
)

type ImportYamlOutput struct {
	OutputMessage string `json:"outputMessage,omitempty" yaml:"outputMessage,omitempty"`
}
