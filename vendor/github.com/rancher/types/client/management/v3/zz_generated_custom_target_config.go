package client

const (
	CustomTargetConfigType         = "customTargetConfig"
	CustomTargetConfigFieldContent = "content"
)

type CustomTargetConfig struct {
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
}
