package client

const (
	GithubLoginInputType      = "githubLoginInput"
	GithubLoginInputFieldCode = "code"
)

type GithubLoginInput struct {
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
}
