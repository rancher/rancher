package client

const (
	GitlabPipelineConfigApplyInputType              = "gitlabPipelineConfigApplyInput"
	GitlabPipelineConfigApplyInputFieldCode         = "code"
	GitlabPipelineConfigApplyInputFieldGitlabConfig = "gitlabConfig"
)

type GitlabPipelineConfigApplyInput struct {
	Code         string                `json:"code,omitempty" yaml:"code,omitempty"`
	GitlabConfig *GitlabPipelineConfig `json:"gitlabConfig,omitempty" yaml:"gitlabConfig,omitempty"`
}
