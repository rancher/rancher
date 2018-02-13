package client

const (
	StepType                    = "step"
	StepFieldPublishImageConfig = "publishImageConfig"
	StepFieldRunScriptConfig    = "runScriptConfig"
	StepFieldSourceCodeConfig   = "sourceCodeConfig"
	StepFieldTimeout            = "timeout"
)

type Step struct {
	PublishImageConfig *PublishImageConfig `json:"publishImageConfig,omitempty"`
	RunScriptConfig    *RunScriptConfig    `json:"runScriptConfig,omitempty"`
	SourceCodeConfig   *SourceCodeConfig   `json:"sourceCodeConfig,omitempty"`
	Timeout            *int64              `json:"timeout,omitempty"`
}
