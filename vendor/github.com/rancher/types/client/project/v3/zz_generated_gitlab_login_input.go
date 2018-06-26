package client

const (
	GitlabLoginInputType      = "gitlabLoginInput"
	GitlabLoginInputFieldCode = "code"
)

type GitlabLoginInput struct {
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
}
