package client

const (
	ProjectMetricNamesInputType             = "projectMetricNamesInput"
	ProjectMetricNamesInputFieldProjectName = "projectId"
)

type ProjectMetricNamesInput struct {
	ProjectName string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
