package client

const (
	UpdateMultiClusterAppTargetsInputType          = "updateMultiClusterAppTargetsInput"
	UpdateMultiClusterAppTargetsInputFieldAnswers  = "answers"
	UpdateMultiClusterAppTargetsInputFieldProjects = "projects"
)

type UpdateMultiClusterAppTargetsInput struct {
	Answers  []Answer `json:"answers,omitempty" yaml:"answers,omitempty"`
	Projects []string `json:"projects,omitempty" yaml:"projects,omitempty"`
}
