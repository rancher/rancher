package client

const (
	PushPipelineConfigInputType         = "pushPipelineConfigInput"
	PushPipelineConfigInputFieldConfigs = "configs"
)

type PushPipelineConfigInput struct {
	Configs map[string]PipelineConfig `json:"configs,omitempty" yaml:"configs,omitempty"`
}
