package client

const (
	GithubPipelineConfigApplyInputType              = "githubPipelineConfigApplyInput"
	GithubPipelineConfigApplyInputFieldCode         = "code"
	GithubPipelineConfigApplyInputFieldGithubConfig = "githubConfig"
	GithubPipelineConfigApplyInputFieldInheritAuth  = "inheritAuth"
)

type GithubPipelineConfigApplyInput struct {
	Code         string                `json:"code,omitempty" yaml:"code,omitempty"`
	GithubConfig *GithubPipelineConfig `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
	InheritAuth  bool                  `json:"inheritAuth,omitempty" yaml:"inheritAuth,omitempty"`
}
