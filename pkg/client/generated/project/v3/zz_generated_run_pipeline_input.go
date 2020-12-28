package client

const (
	RunPipelineInputType        = "runPipelineInput"
	RunPipelineInputFieldBranch = "branch"
)

type RunPipelineInput struct {
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
}
