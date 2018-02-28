package client

const (
	StepType                    = "step"
	StepFieldPublishImageConfig = "publishImageConfig"
	StepFieldRunScriptConfig    = "runScriptConfig"
	StepFieldSourceCodeConfig   = "sourceCodeConfig"
	StepFieldTimeout            = "timeout"
)

type Step struct {
	PublishImageConfig *PublishImageConfig `json:"publishImageConfig,omitempty" yaml:"publishImageConfig,omitempty"`
	RunScriptConfig    *RunScriptConfig    `json:"runScriptConfig,omitempty" yaml:"runScriptConfig,omitempty"`
	SourceCodeConfig   *SourceCodeConfig   `json:"sourceCodeConfig,omitempty" yaml:"sourceCodeConfig,omitempty"`
	Timeout            *int64              `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
